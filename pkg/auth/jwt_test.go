package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestGenerateAndValidateToken(t *testing.T) {
	secret := "test-secret"
	userID := uuid.Must(uuid.NewV7()).String()

	token, err := GenerateToken(secret, userID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims, err := ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID mismatch: got %s, want %s", claims.UserID, userID)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	secretA := "secret-a"
	secretB := "secret-b"
	userID := uuid.Must(uuid.NewV7()).String()

	token, err := GenerateToken(secretA, userID)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(secretB, token)
	if err == nil {
		t.Error("expected error when validating with wrong secret, got nil")
	}
}

func TestValidateToken_MalformedToken(t *testing.T) {
	secret := "test-secret"

	_, err := ValidateToken(secret, "not-a-token")
	if err == nil {
		t.Error("expected error for malformed token, got nil")
	}
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
	if err != nil {
		t.Fatalf("failed to create expired token: %v", err)
	}

	_, err = ValidateToken(secret, tokenString)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}
