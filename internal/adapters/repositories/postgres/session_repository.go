package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// HashToken calcula el hash criptográfico SHA-256 en formato hexadecimal de un token de sesión.
// Permite almacenar e indexar tokens en la base de datos sin persistir el token original en texto plano.
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// PostgresSessionRepository implementa ports.SessionRepository respaldado por la tabla `sessions` de PostgreSQL.
// Permite escalado horizontal multi-réplica, persistencia ante reinicios e invalidación global de sesiones.
type PostgresSessionRepository struct {
	db DBExecutor
}

// NewSessionRepository crea un repositorio de sesiones distribuido respaldado por PostgreSQL.
func NewSessionRepository(db DBExecutor) *PostgresSessionRepository {
	return &PostgresSessionRepository{db: db}
}

func (r *PostgresSessionRepository) Save(ctx context.Context, session *domain.Session, ttl time.Duration) error {
	if session == nil {
		return fmt.Errorf("%w: la sesión no puede ser nula", domain.ErrInvalidInput)
	}
	if session.ID == "" || session.UserID == "" || session.Token == "" {
		return fmt.Errorf("%w: id, user_id y token de sesión son obligatorios", domain.ErrInvalidInput)
	}

	now := time.Now().UTC()
	createdAt := session.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}

	expiresAt := session.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = createdAt.Add(ttl)
	}

	tokenHash := HashToken(session.Token)

	query := `
		INSERT INTO sessions (
			id, user_id, token, user_agent, client_ip, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			token = EXCLUDED.token,
			user_agent = EXCLUDED.user_agent,
			client_ip = EXCLUDED.client_ip,
			expires_at = EXCLUDED.expires_at
	`
	_, err := executor(ctx, r.db).Exec(ctx, query,
		session.ID,
		session.UserID,
		tokenHash,
		session.UserAgent,
		session.ClientIP,
		expiresAt,
		createdAt,
	)
	if err != nil {
		return fmt.Errorf("error al guardar sesión en PostgreSQL: %w", err)
	}

	return nil
}

func (r *PostgresSessionRepository) GetByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	if sessionID == "" {
		return nil, domain.ErrSessionExpired
	}

	query := `
		SELECT id, user_id, token, user_agent, client_ip, expires_at, created_at
		FROM sessions
		WHERE id = $1 AND expires_at > NOW()
	`
	return r.scanSession(executor(ctx, r.db).QueryRow(ctx, query, sessionID))
}

func (r *PostgresSessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	if token == "" {
		return nil, domain.ErrSessionExpired
	}

	tokenHash := HashToken(token)

	query := `
		SELECT id, user_id, token, user_agent, client_ip, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > NOW()
	`
	return r.scanSession(executor(ctx, r.db).QueryRow(ctx, query, tokenHash))
}

func (r *PostgresSessionRepository) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	query := `DELETE FROM sessions WHERE id = $1`
	if _, err := executor(ctx, r.db).Exec(ctx, query, sessionID); err != nil {
		return fmt.Errorf("error al eliminar sesión de PostgreSQL: %w", err)
	}

	return nil
}

func (r *PostgresSessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	if userID == "" {
		return nil
	}

	query := `DELETE FROM sessions WHERE user_id = $1`
	if _, err := executor(ctx, r.db).Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("error al eliminar sesiones del usuario en PostgreSQL: %w", err)
	}

	return nil
}

// PurgeExpired elimina en PostgreSQL las sesiones cuyo TTL ya expiró.
func (r *PostgresSessionRepository) PurgeExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM sessions WHERE expires_at <= NOW()`
	tag, err := executor(ctx, r.db).Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("error al depurar sesiones expiradas: %w", err)
	}
	return tag.RowsAffected(), nil
}

// StartCleanup ejecuta un bucle periódico de limpieza de sesiones expiradas hasta que ctx se cancele.
func (r *PostgresSessionRepository) StartCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purgeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if _, err := r.PurgeExpired(purgeCtx); err != nil {
				log.Printf("[WARN] Error depurando sesiones expiradas en PostgreSQL: %v", err)
			}
			cancel()
		}
	}
}

func (r *PostgresSessionRepository) scanSession(row pgx.Row) (*domain.Session, error) {
	var s domain.Session
	err := row.Scan(
		&s.ID,
		&s.UserID,
		&s.Token,
		&s.UserAgent,
		&s.ClientIP,
		&s.ExpiresAt,
		&s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrSessionExpired
		}
		return nil, fmt.Errorf("error escaneando fila de sesión: %w", err)
	}
	return &s, nil
}

// Asegurar que PostgresSessionRepository implementa ports.SessionRepository
var _ ports.SessionRepository = (*PostgresSessionRepository)(nil)

type sessionItem struct {
	session   *domain.Session
	expiresAt time.Time
}

// MemorySessionRepository almacena sesiones en memoria (uso exclusivo para pruebas unitarias/HTTP aisladas).
// En entornos de producción debe utilizarse PostgresSessionRepository (NewSessionRepository).
type MemorySessionRepository struct {
	mu          sync.RWMutex
	byID        map[string]sessionItem
	byToken     map[string]string              // sha256(token) -> sessionID
	byUserID    map[string]map[string]struct{} // userID -> set of sessionIDs
	stopCleanup chan struct{}
}

// NewMemorySessionRepository crea un repositorio en memoria para pruebas aisladas con soporte de TTL.
func NewMemorySessionRepository(cleanupInterval time.Duration) *MemorySessionRepository {
	repo := &MemorySessionRepository{
		byID:        make(map[string]sessionItem),
		byToken:     make(map[string]string),
		byUserID:    make(map[string]map[string]struct{}),
		stopCleanup: make(chan struct{}),
	}

	if cleanupInterval > 0 {
		go repo.startCleanupLoop(cleanupInterval)
	}

	return repo
}

func (r *MemorySessionRepository) Save(ctx context.Context, session *domain.Session, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	expiresAt := time.Now().UTC().Add(ttl)
	if !session.ExpiresAt.IsZero() {
		expiresAt = session.ExpiresAt
	}

	tokenHash := HashToken(session.Token)
	stored := *session
	stored.Token = tokenHash

	// Guardar sesión por ID
	r.byID[session.ID] = sessionItem{
		session:   &stored,
		expiresAt: expiresAt,
	}

	// Indexar por hash SHA-256 del token
	r.byToken[tokenHash] = session.ID

	// Indexar por UserID
	if _, exists := r.byUserID[session.UserID]; !exists {
		r.byUserID[session.UserID] = make(map[string]struct{})
	}
	r.byUserID[session.UserID][session.ID] = struct{}{}

	return nil
}

func (r *MemorySessionRepository) GetByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	item, exists := r.byID[sessionID]
	if !exists {
		return nil, domain.ErrSessionExpired
	}

	if time.Now().UTC().After(item.expiresAt) {
		return nil, domain.ErrSessionExpired
	}

	return item.session, nil
}

func (r *MemorySessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tokenHash := HashToken(token)
	sessionID, exists := r.byToken[tokenHash]
	if !exists {
		return nil, domain.ErrSessionExpired
	}

	item, exists := r.byID[sessionID]
	if !exists {
		return nil, domain.ErrSessionExpired
	}

	if time.Now().UTC().After(item.expiresAt) {
		return nil, domain.ErrSessionExpired
	}

	return item.session, nil
}

func (r *MemorySessionRepository) Delete(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if item, exists := r.byID[sessionID]; exists {
		delete(r.byToken, item.session.Token)
		if set, ok := r.byUserID[item.session.UserID]; ok {
			delete(set, sessionID)
			if len(set) == 0 {
				delete(r.byUserID, item.session.UserID)
			}
		}
		delete(r.byID, sessionID)
	}

	return nil
}

func (r *MemorySessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if sessionIDs, exists := r.byUserID[userID]; exists {
		for sessID := range sessionIDs {
			if item, ok := r.byID[sessID]; ok {
				delete(r.byToken, item.session.Token)
				delete(r.byID, sessID)
			}
		}
		delete(r.byUserID, userID)
	}

	return nil
}

func (r *MemorySessionRepository) Close() {
	close(r.stopCleanup)
}

func (r *MemorySessionRepository) startCleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.purgeExpired()
		case <-r.stopCleanup:
			return
		}
	}
}

func (r *MemorySessionRepository) purgeExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for id, item := range r.byID {
		if now.After(item.expiresAt) {
			delete(r.byToken, item.session.Token)
			if set, ok := r.byUserID[item.session.UserID]; ok {
				delete(set, id)
				if len(set) == 0 {
					delete(r.byUserID, item.session.UserID)
				}
			}
			delete(r.byID, id)
		}
	}
}

// Asegurar que MemorySessionRepository implementa ports.SessionRepository
var _ ports.SessionRepository = (*MemorySessionRepository)(nil)
