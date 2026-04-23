package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/felipersas/payflow/internal/user/application/commands"
	"github.com/felipersas/payflow/internal/user/domain/entities"
	"github.com/felipersas/payflow/internal/user/domain/repositories"
	"github.com/felipersas/payflow/pkg/auth"
	gomock "go.uber.org/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupService(t *testing.T) (*AuthService, *repositories.MockUserRepository) {
	ctrl := gomock.NewController(t)
	mockRepo := repositories.NewMockUserRepository(ctrl)
	jwtSecret := "test-secret-key-at-least-32b"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewAuthService(mockRepo, jwtSecret, logger), mockRepo
}

func TestRegister_Valid(t *testing.T) {
	svc, mockRepo := setupService(t)

	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(nil, nil)
	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	result, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	require.NotNil(t, result.User)
	assert.NotEmpty(t, result.User.ID)
	assert.Equal(t, "test@test.com", result.User.Email)
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, mockRepo := setupService(t)

	existingUser, _ := entities.NewUser("test@test.com", "$2a$10$hash")
	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(existingUser, nil)

	_, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "otherpass456",
	})
	require.Error(t, err)
	assert.Equal(t, "email already registered", err.Error())
}

func TestLogin_Valid(t *testing.T) {
	svc, mockRepo := setupService(t)

	// bcrypt hash of "password123" - generated with bcrypt.DefaultCost
	hashedPassword := "$2a$10$fsW1ppYmnRPpl1TJQnL.DOx1SvmCsAJtSVZBIbOJ8EwL4M/PW4kFm"
	user, _ := entities.NewUser("test@test.com", hashedPassword)
	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(user, nil)

	result, err := svc.Login(context.Background(), commands.LoginCommand{
		Email:    "test@test.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Token)
	require.NotNil(t, result.User)
	assert.Equal(t, "test@test.com", result.User.Email)
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, mockRepo := setupService(t)

	// Create a user with a pre-hashed password for "correctpassword"
	// $2a$10$... is bcrypt hash for "correctpassword"
	hashedPassword := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	user, _ := entities.NewUser("test@test.com", hashedPassword)
	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(user, nil)

	_, err := svc.Login(context.Background(), commands.LoginCommand{
		Email:    "test@test.com",
		Password: "wrongpassword",
	})
	require.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())
}

func TestLogin_NonexistentEmail(t *testing.T) {
	svc, mockRepo := setupService(t)

	mockRepo.EXPECT().GetByEmail(gomock.Any(), "nonexistent@test.com").Return(nil, fmt.Errorf("not found"))

	_, err := svc.Login(context.Background(), commands.LoginCommand{
		Email:    "nonexistent@test.com",
		Password: "password123",
	})
	require.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())
}

func TestRegister_TokenRoundTrip(t *testing.T) {
	svc, mockRepo := setupService(t)
	jwtSecret := "test-secret-key-at-least-32b"

	mockRepo.EXPECT().GetByEmail(gomock.Any(), "test@test.com").Return(nil, nil)
	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)

	result, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "password123",
	})
	require.NoError(t, err)

	claims, err := auth.ValidateToken(jwtSecret, result.Token)
	require.NoError(t, err)
	assert.Equal(t, result.User.ID, claims.UserID)
}
