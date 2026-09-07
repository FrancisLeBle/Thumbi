package postgres

import (
	"context"
	"sync"
	"time"

	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type sessionItem struct {
	session   *domain.Session
	expiresAt time.Time
}

// MemorySessionRepository almacena sesiones en memoria/cache con expiración estricta y limpieza automática de TTL
type MemorySessionRepository struct {
	mu           sync.RWMutex
	byID         map[string]sessionItem
	byToken      map[string]string // token -> sessionID
	byUserID     map[string]map[string]struct{} // userID -> set of sessionIDs
	stopCleanup  chan struct{}
}

// NewMemorySessionRepository crea un nuevo repositorio de sesiones con soporte de TTL
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

	// Guardar sesión por ID
	r.byID[session.ID] = sessionItem{
		session:   session,
		expiresAt: expiresAt,
	}

	// Indexar por Token
	r.byToken[session.Token] = session.ID

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

	sessionID, exists := r.byToken[token]
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
