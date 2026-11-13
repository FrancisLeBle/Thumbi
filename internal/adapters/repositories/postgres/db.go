package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// DBExecutor define la interfaz mínima compatible con *pgxpool.Pool, *pgx.Conn o pgx.Tx
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxBeginner define la capacidad de iniciar una transacción pgx.Tx (implementado por *pgxpool.Pool y *pgx.Conn)
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type txContextKey struct{}

// WithTx inyecta una transacción pgx.Tx activa en el contexto para coordinar múltiples repositorios atómicamente.
func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
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

type pgxTransactionManager struct {
	beginner TxBeginner
}

// NewTransactionManager construye un gestor de transacciones ACID respaldado por PostgreSQL/pgx.
func NewTransactionManager(beginner TxBeginner) ports.TransactionManager {
	return &pgxTransactionManager{beginner: beginner}
}

// NewUnitOfWork construye una implementación PostgreSQL del puerto ports.UnitOfWork.
func NewUnitOfWork(beginner TxBeginner) ports.UnitOfWork {
	return NewTransactionManager(beginner)
}

// WithinTransaction ejecuta fn dentro de una única transacción pgx.Tx, realizando Rollback automático ante errores o panics.
func (m *pgxTransactionManager) WithinTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	if txFromContext(ctx) != nil {
		return fn(ctx)
	}
	if m.beginner == nil {
		return fmt.Errorf("error iniciando transacción: TxBeginner no configurado")
	}

	tx, err := m.beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("error iniciando transacción de base de datos: %w", err)
	}

	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	txCtx := WithTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("error confirmando transacción de base de datos: %w", err)
	}
	tx = nil
	return nil
}

