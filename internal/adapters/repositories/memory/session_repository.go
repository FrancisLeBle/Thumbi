package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

func hashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

type sessionItem struct {
	session   *domain.Session
	expiresAt time.Time
}

// SessionRepository almacena sesiones activas en memoria (uso exclusivo para pruebas aisladas o desarrollo local).
// En entornos productivos multi-réplica debe utilizarse postgres.NewSessionRepository.
type SessionRepository struct {
	mu          sync.RWMutex
	byID        map[uuid.UUID]sessionItem
	byToken     map[string]uuid.UUID // sha256(token) -> sessionID
	stopCleanup chan struct{}
	closeOnce   sync.Once
}

// NewSessionRepository crea un repositorio de sesiones en memoria con soporte de TTL y depuración periódica opcional.
func NewSessionRepository(cleanupInterval time.Duration) *SessionRepository {
	repo := &SessionRepository{
		byID:        make(map[uuid.UUID]sessionItem),
		byToken:     make(map[string]uuid.UUID),
		stopCleanup: make(chan struct{}),
	}

	if cleanupInterval > 0 {
		go repo.startCleanupLoop(cleanupInterval)
	}

	return repo
}

// Save persiste o actualiza la sesión indexándola por ID y hash SHA-256 del token.
func (r *SessionRepository) Save(ctx context.Context, session *domain.Session, ttl time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	expiresAt := time.Now().UTC().Add(ttl)
	if !session.ExpiresAt.IsZero() {
		expiresAt = session.ExpiresAt
	}

	tokenHash := hashToken(session.Token)
	stored := *session
	stored.Token = tokenHash

	r.byID[session.ID] = sessionItem{
		session:   &stored,
		expiresAt: expiresAt,
	}
	r.byToken[tokenHash] = session.ID

	return nil
}

// GetByToken recupera una sesión activa buscando por el hash SHA-256 del token.
func (r *SessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tokenHash := hashToken(token)
	sessionID, exists := r.byToken[tokenHash]
	if !exists {
		return nil, domain.ErrSessionExpired
	}

	item, exists := r.byID[sessionID]
	if !exists || time.Now().UTC().After(item.expiresAt) {
		return nil, domain.ErrSessionExpired
	}

	return item.session, nil
}

// Delete invalida y elimina una sesión por su ID.
func (r *SessionRepository) Delete(ctx context.Context, sessionID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if item, exists := r.byID[sessionID]; exists {
		delete(r.byToken, item.session.Token)
		delete(r.byID, sessionID)
	}

	return nil
}

// Close detiene la goroutine de limpieza periódica de forma idempotente.
func (r *SessionRepository) Close() {
	r.closeOnce.Do(func() {
		close(r.stopCleanup)
	})
}

func (r *SessionRepository) startCleanupLoop(interval time.Duration) {
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

func (r *SessionRepository) purgeExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for id, item := range r.byID {
		if now.After(item.expiresAt) {
			delete(r.byToken, item.session.Token)
			delete(r.byID, id)
		}
	}
}

// Asegurar que SessionRepository implementa ports.SessionRepository
var _ ports.SessionRepository = (*SessionRepository)(nil)
