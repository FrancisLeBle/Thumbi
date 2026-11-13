package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/adapters/repositories/postgres"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
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

	t.Run("almacena el hash SHA-256 del token y nunca el token en texto plano", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		session := &domain.Session{
			ID:        "sess-101",
			UserID:    "user-202",
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
				args[0] == "sess-101" &&
				args[1] == "user-202" &&
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

		err = repo.Save(ctx, &domain.Session{ID: "s1", UserID: "u1"}, domain.SessionTTL)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	})

	t.Run("propaga errores de PostgreSQL", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		session := domain.NewSession("sess-err", "user-1", rawToken, "", "")
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

	t.Run("busca por hash SHA-256 del token verificando expires_at > NOW()", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM sessions") &&
				strings.Contains(sql, "WHERE token = $1") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{tokenHash}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*string) = "sess-1"
				*dest[1].(*string) = "user-1"
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
		assert.Equal(t, "sess-1", sess.ID)
		assert.Equal(t, "user-1", sess.UserID)
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

func TestPostgresSessionRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	expiresAt := now.Add(10 * time.Minute)
	tokenHash := postgres.HashToken("some-jwt")

	t.Run("recupera sesión activa por ID validando TTL", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM sessions") &&
				strings.Contains(sql, "WHERE id = $1") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{"sess-99"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*string) = "sess-99"
				*dest[1].(*string) = "user-99"
				*dest[2].(*string) = tokenHash
				*dest[3].(*string) = "Safari"
				*dest[4].(*string) = "10.0.0.5"
				*dest[5].(*time.Time) = expiresAt
				*dest[6].(*time.Time) = now
				return nil
			},
		})

		sess, err := repo.GetByID(ctx, "sess-99")
		require.NoError(t, err)
		assert.Equal(t, "sess-99", sess.ID)
		assert.Equal(t, "user-99", sess.UserID)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrSessionExpired cuando no hay filas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{"missing"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		sess, err := repo.GetByID(ctx, "missing")
		assert.ErrorIs(t, err, domain.ErrSessionExpired)
		assert.Nil(t, sess)
		mockDB.AssertExpectations(t)
	})
}

func TestPostgresSessionRepository_DeleteAndPurge(t *testing.T) {
	ctx := context.Background()

	t.Run("Delete elimina la sesión globalmente por ID", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "DELETE FROM sessions") && strings.Contains(sql, "WHERE id = $1")
		}), []any{"sess-1"}).Return(pgconn.NewCommandTag("DELETE 1"), nil)

		err := repo.Delete(ctx, "sess-1")
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("DeleteByUserID invalida todas las sesiones del usuario en el cluster", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "DELETE FROM sessions") && strings.Contains(sql, "WHERE user_id = $1")
		}), []any{"user-77"}).Return(pgconn.NewCommandTag("DELETE 3"), nil)

		err := repo.DeleteByUserID(ctx, "user-77")
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("PurgeExpired elimina sesiones expiradas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewSessionRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "DELETE FROM sessions") && strings.Contains(sql, "expires_at <= NOW()")
		}), mock.Anything).Return(pgconn.NewCommandTag("DELETE 5"), nil)

		deleted, err := repo.PurgeExpired(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(5), deleted)
		mockDB.AssertExpectations(t)
	})
}

func TestMemorySessionRepository_HashedLookup(t *testing.T) {
	ctx := context.Background()
	repo := postgres.NewMemorySessionRepository(0)
	defer repo.Close()

	rawToken := "secret-token-123"
	sess := domain.NewSession("s-1", "u-1", rawToken, "Agent", "1.2.3.4")

	require.NoError(t, repo.Save(ctx, sess, domain.SessionTTL))

	found, err := repo.GetByToken(ctx, rawToken)
	require.NoError(t, err)
	assert.Equal(t, "s-1", found.ID)
	assert.Equal(t, postgres.HashToken(rawToken), found.Token)
	assert.Equal(t, rawToken, sess.Token)

	require.NoError(t, repo.DeleteByUserID(ctx, "u-1"))
	_, err = repo.GetByToken(ctx, rawToken)
	assert.ErrorIs(t, err, domain.ErrSessionExpired)
}
