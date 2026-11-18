package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/thumbi/backend/internal/core/ports"
)

// DBExecutor define la interfaz mínima compatible con *pgxpool.Pool, *pgx.Conn o pgx.Tx.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxBeginner define la capacidad de iniciar una transacción pgx.Tx (implementado por *pgxpool.Pool y *pgx.Conn).
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// DBPool combina ejecución SQL e inicio explícito de transacciones ACID.
type DBPool interface {
	DBExecutor
	TxBeginner
}

// TxUnitOfWork extiende el puerto ports.UnitOfWork para la capa de persistencia PostgreSQL/pgx,
// exponiendo WithTx(ctx, func(tx pgx.Tx) error) respaldado por pgx.BeginFunc.
type TxUnitOfWork interface {
	ports.UnitOfWork
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

type txContextKey struct{}

// ContextWithTx inyecta una transacción pgx.Tx activa en el contexto para coordinar múltiples repositorios atómicamente.
func ContextWithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

func txFromContext(ctx context.Context) pgx.Tx {
	if ctx == nil {
		return nil
	}
	tx, _ := ctx.Value(txContextKey{}).(pgx.Tx)
	return tx
}

// executor retorna la transacción activa presente en ctx si existe, o el DBExecutor base en caso contrario.
func executor(ctx context.Context, fallback DBExecutor) DBExecutor {
	if tx := txFromContext(ctx); tx != nil {
		return tx
	}
	return fallback
}

// WithTx ejecuta fn dentro de una única transacción pgx.Tx utilizando pgx.BeginFunc.
// Si ctx ya transporta una transacción activa (por ejemplo desde UnitOfWork.WithinTransaction),
// la reutiliza preservando la atomicidad. Nunca degrada silenciosamente a ejecución no transaccional.
func WithTx(ctx context.Context, beginner TxBeginner, fn func(tx pgx.Tx) error) error {
	if fn == nil {
		return fmt.Errorf("error iniciando transacción: función transaccional nula")
	}
	if existingTx := txFromContext(ctx); existingTx != nil {
		return fn(existingTx)
	}
	if beginner == nil {
		return fmt.Errorf("error iniciando transacción: TxBeginner no configurado")
	}
	return pgx.BeginFunc(ctx, beginner, fn)
}

type pgxTransactionManager struct {
	beginner TxBeginner
}

// NewUnitOfWork construye la implementación PostgreSQL de TxUnitOfWork / ports.UnitOfWork.
func NewUnitOfWork(beginner TxBeginner) TxUnitOfWork {
	return &pgxTransactionManager{beginner: beginner}
}

// NewTransactionManager construye un gestor de transacciones ACID respaldado por PostgreSQL/pgx.
func NewTransactionManager(beginner TxBeginner) ports.TransactionManager {
	return NewUnitOfWork(beginner)
}

// WithTx ejecuta fn dentro de una transacción pgx.Tx gestionada por pgx.BeginFunc.
func (m *pgxTransactionManager) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return WithTx(ctx, m.beginner, fn)
}

// WithinTransaction ejecuta fn dentro de una única transacción pgx.Tx propagada en el contexto,
// realizando Rollback automático ante errores o panics mediante pgx.BeginFunc.
func (m *pgxTransactionManager) WithinTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	if fn == nil {
		return fmt.Errorf("error iniciando transacción: callback nulo")
	}
	return m.WithTx(ctx, func(tx pgx.Tx) error {
		return fn(ContextWithTx(ctx, tx))
	})
}

func resolveTxUnitOfWork(db DBExecutor, optionalUOW ...TxUnitOfWork) TxUnitOfWork {
	if len(optionalUOW) > 0 && optionalUOW[0] != nil {
		return optionalUOW[0]
	}
	if beginner, ok := db.(TxBeginner); ok && beginner != nil {
		return NewUnitOfWork(beginner)
	}
	return nil
}

func runInRepoTx(ctx context.Context, uow TxUnitOfWork, fn func(tx pgx.Tx) error) error {
	if fn == nil {
		return fmt.Errorf("error ejecutando operación transaccional: callback nulo")
	}
	if existingTx := txFromContext(ctx); existingTx != nil {
		return fn(existingTx)
	}
	if uow == nil {
		return fmt.Errorf("error de integridad transaccional: UnitOfWork/TxBeginner no configurado")
	}
	return uow.WithTx(ctx, fn)
}
