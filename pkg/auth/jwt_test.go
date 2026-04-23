package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret"
	userID := uuid.Must(uuid.NewV7()).String()

	token, err := GenerateToken(secret, userID)
	require.NoError(t, err, "GenerateToken failed")
	assert.NotEmpty(t, token, "token is empty")

	claims, err := ValidateToken(secret, token)
	require.NoError(t, err, "ValidateToken failed")
	assert.Equal(t, userID, claims.UserID, "UserID mismatch")
}

func TestValidateToken_WrongSecret(t *testing.T) {
	secretA := "secret-a"
	secretB := "secret-b"
	userID := uuid.Must(uuid.NewV7()).String()

	token, err := GenerateToken(secretA, userID)
	require.NoError(t, err, "GenerateToken failed")

	_, err = ValidateToken(secretB, token)
	assert.Error(t, err, "expected error when validating with wrong secret, got nil")
}

func TestValidateToken_MalformedToken(t *testing.T) {
	secret := "test-secret"

	_, err := ValidateToken(secret, "not-a-token")
	assert.Error(t, err, "expected error for malformed token, got nil")
}

func TestValidateToken_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	userID := uuid.Must(uuid.NewV7()).String()

	// Manually create an expired token
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ID:        uuid.Must(uuid.NewV7()).String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err, "failed to create expired token")

	_, err = ValidateToken(secret, tokenString)
	assert.Error(t, err, "expected error for expired token, got nil")
}
