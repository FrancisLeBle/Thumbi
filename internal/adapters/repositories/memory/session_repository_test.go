package memory_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/repositories/memory"
	"github.com/thumbi/backend/internal/core/domain"
)

func expectedSHA256(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func TestSessionRepository_HashedLookupAndInvalidation(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewSessionRepository(0)
	defer repo.Close()

	sessID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	userID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")
	rawToken := "secret-token-123"
	sess := domain.NewSession(sessID, userID, rawToken, "Agent", "1.2.3.4")

	require.NoError(t, repo.Save(ctx, sess, domain.SessionTTL))

	found, err := repo.GetByToken(ctx, rawToken)
	require.NoError(t, err)
	assert.Equal(t, sessID, found.ID)
	assert.Equal(t, expectedSHA256(rawToken), found.Token)
	assert.Equal(t, rawToken, sess.Token)

	require.NoError(t, repo.Delete(ctx, sessID))
	_, err = repo.GetByToken(ctx, rawToken)
	assert.ErrorIs(t, err, domain.ErrSessionExpired)
}

func TestSessionRepository_ExpirationAndDelete(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewSessionRepository(0)
	defer repo.Close()

	sessID := uuid.MustParse("01933e3d-0000-7000-8000-000000000010")
	userID := uuid.MustParse("01933e3d-0000-7000-8000-000000000020")
	expiredSess := &domain.Session{
		ID:        sessID,
		UserID:    userID,
		Token:     "expired-token",
		CreatedAt: time.Now().UTC().Add(-25 * time.Minute),
		ExpiresAt: time.Now().UTC().Add(-5 * time.Minute),
	}

	require.NoError(t, repo.Save(ctx, expiredSess, domain.SessionTTL))

	_, err := repo.GetByToken(ctx, "expired-token")
	assert.ErrorIs(t, err, domain.ErrSessionExpired)

	require.NoError(t, repo.Delete(ctx, sessID))
}
