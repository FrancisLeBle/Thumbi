package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/repositories/postgres"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestHashToken(t *testing.T) {
	t.Run("genera digest SHA-256 determinístico en hexadecimal de 64 caracteres", func(t *testing.T) {
		raw := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig"
		h1 := postgres.HashToken(raw)
		h2 := postgres.HashToken(raw)
		h3 := postgres.HashToken(raw + "-other")

		assert.Len(t, h1, 64)
		assert.Equal(t, h1, h2)
		assert.NotEqual(t, raw, h1)
		assert.NotEqual(t, h1, h3)
	})
}

func TestPostgresSessionRepository_Save(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	rawToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test-raw-token"
	expectedHash := postgres.HashToken(rawToken)
	sessID := uuid.MustParse("01933e3d-0000-7000-8000-000000000101")
	userID := uuid.MustParse("01933e3d-0000-7000-8000-000000000202")

	t.Run("almacena el hash SHA-256 del token y nunca el token en texto plano", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		session := &domain.Session{
			ID:        sessID,
			UserID:    userID,
			Token:     rawToken,
			UserAgent: "ThumbiApp/iOS",
			ClientIP:  "190.18.20.1",
			CreatedAt: now,
			ExpiresAt: now.Add(domain.SessionTTL),
		}

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO sessions") &&
				strings.Contains(sql, "ON CONFLICT (id) DO UPDATE")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 7 &&
				args[0] == sessID &&
				args[1] == userID &&
				args[2] == expectedHash &&
				args[2] != rawToken &&
				args[3] == "ThumbiApp/iOS" &&
				args[4] == "190.18.20.1"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err := repo.Save(ctx, session, domain.SessionTTL)
		require.NoError(t, err)
		assert.Equal(t, rawToken, session.Token, "No debe mutar el puntero original del llamador")
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrInvalidInput si la sesión es nula o faltan campos requeridos", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		err := repo.Save(ctx, nil, domain.SessionTTL)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)

		err = repo.Save(ctx, &domain.Session{ID: sessID, UserID: userID}, domain.SessionTTL)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("propaga errores de PostgreSQL", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		session := domain.NewSession(sessID, userID, rawToken, "", "")
		mockDB.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, errors.New("connection refused"))

		err := repo.Save(ctx, session, domain.SessionTTL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error al guardar sesión")
		mockDB.AssertExpectations(t)
	})
}

func TestPostgresSessionRepository_GetByToken(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Minute)
	rawToken := "raw-jwt-token-xyz"
	tokenHash := postgres.HashToken(rawToken)
	sessID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	userID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")

	t.Run("busca por hash SHA-256 del token verificando expires_at > NOW()", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM sessions") &&
				strings.Contains(sql, "WHERE token = $1") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{tokenHash}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = sessID
				*dest[1].(*uuid.UUID) = userID
				*dest[2].(*string) = tokenHash
				*dest[3].(*string) = "Chrome/120"
				*dest[4].(*string) = "127.0.0.1"
				*dest[5].(*time.Time) = expiresAt
				*dest[6].(*time.Time) = now
				return nil
			},
		})

		sess, err := repo.GetByToken(ctx, rawToken)
		require.NoError(t, err)
		require.NotNil(t, sess)
		assert.Equal(t, sessID, sess.ID)
		assert.Equal(t, userID, sess.UserID)
		assert.Equal(t, tokenHash, sess.Token)
		assert.Equal(t, expiresAt, sess.ExpiresAt)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrSessionExpired cuando la sesión no existe o venció su TTL", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{tokenHash}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		sess, err := repo.GetByToken(ctx, rawToken)
		assert.ErrorIs(t, err, domain.ErrSessionExpired)
		assert.Nil(t, sess)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrSessionExpired inmediatamente si el token está vacío", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		sess, err := repo.GetByToken(ctx, "")
		assert.ErrorIs(t, err, domain.ErrSessionExpired)
		assert.Nil(t, sess)
	})
}

func TestPostgresSessionRepository_DeleteAndPurge(t *testing.T) {
	ctx := context.Background()
	sessID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")

	t.Run("Delete elimina la sesión globalmente por ID", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "DELETE FROM sessions") && strings.Contains(sql, "WHERE id = $1")
		}), []any{sessID}).Return(pgconn.NewCommandTag("DELETE 1"), nil)

		err := repo.Delete(ctx, sessID)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("PurgeExpired elimina sesiones expiradas en lotes usando FOR UPDATE SKIP LOCKED", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "DELETE FROM sessions") &&
				strings.Contains(sql, "expires_at <= NOW()") &&
				strings.Contains(sql, "FOR UPDATE SKIP LOCKED")
		}), mock.Anything).Return(pgconn.NewCommandTag("DELETE 5"), nil)

		deleted, err := repo.PurgeExpired(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(5), deleted)
		mockDB.AssertExpectations(t)
	})
}
