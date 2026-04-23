package services

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/felipersas/payflow/internal/user/application/commands"
	"github.com/felipersas/payflow/internal/user/domain/entities"
	"github.com/felipersas/payflow/pkg/auth"
)

type mockUserRepo struct {
	usersByEmail map[string]*entities.User
	usersByID    map[string]*entities.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByEmail: make(map[string]*entities.User),
		usersByID:    make(map[string]*entities.User),
	}
}

func (m *mockUserRepo) Create(_ context.Context, user *entities.User) error {
	m.usersByEmail[user.Email] = user
	m.usersByID[user.ID] = user
	return nil
}

func (m *mockUserRepo) GetByEmail(_ context.Context, email string) (*entities.User, error) {
	u, ok := m.usersByEmail[email]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return u, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id string) (*entities.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return u, nil
}

func setupService() (*AuthService, *mockUserRepo) {
	repo := newMockUserRepo()
	jwtSecret := "test-secret-key-at-least-32b"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewAuthService(repo, jwtSecret, logger), repo
}

func TestRegister_Valid(t *testing.T) {
	svc, _ := setupService()

	result, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result.Token == "" {
		t.Error("Token should not be empty")
	}
	if result.User == nil {
		t.Fatal("User should not be nil")
	}
	if result.User.ID == "" {
		t.Error("User ID should not be empty")
	}
	if result.User.Email != "test@test.com" {
		t.Errorf("User Email = %q, want %q", result.User.Email, "test@test.com")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _ := setupService()

	_, _ = svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "password123",
	})

	_, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "otherpass456",
	})
	if err == nil {
		t.Error("expected error for duplicate email")
	}
	if err != nil && err.Error() != "email already registered" {
		t.Errorf("error = %q, want %q", err.Error(), "email already registered")
	}
}

func TestRegister_EmptyEmail(t *testing.T) {
	svc, _ := setupService()

	_, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "",
		Password: "password123",
	})
	if err == nil {
		t.Error("expected error for empty email")
	}
}

func TestRegister_EmptyPassword(t *testing.T) {
	svc, _ := setupService()

	_, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "",
	})
	if err == nil {
		t.Error("expected error for empty password")
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	svc, _ := setupService()

	_, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "12345",
	})
	if err == nil {
		t.Error("expected error for short password")
	}
	if err != nil && err.Error() != "password must be at least 6 characters" {
		t.Errorf("error = %q, want %q", err.Error(), "password must be at least 6 characters")
	}
}

func TestLogin_Valid(t *testing.T) {
	svc, _ := setupService()

	_, _ = svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "password123",
	})

	result, err := svc.Login(context.Background(), commands.LoginCommand{
		Email:    "test@test.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.Token == "" {
		t.Error("Token should not be empty")
	}
	if result.User == nil {
		t.Fatal("User should not be nil")
	}
	if result.User.Email != "test@test.com" {
		t.Errorf("User Email = %q, want %q", result.User.Email, "test@test.com")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := setupService()

	_, _ = svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "password123",
	})

	_, err := svc.Login(context.Background(), commands.LoginCommand{
		Email:    "test@test.com",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Error("expected error for wrong password")
	}
	if err != nil && err.Error() != "invalid credentials" {
		t.Errorf("error = %q, want %q", err.Error(), "invalid credentials")
	}
}

func TestLogin_NonexistentEmail(t *testing.T) {
	svc, _ := setupService()

	_, err := svc.Login(context.Background(), commands.LoginCommand{
		Email:    "nonexistent@test.com",
		Password: "password123",
	})
	if err == nil {
		t.Error("expected error for nonexistent email")
	}
	if err != nil && err.Error() != "invalid credentials" {
		t.Errorf("error = %q, want %q", err.Error(), "invalid credentials")
	}
}

func TestRegister_TokenRoundTrip(t *testing.T) {
	svc, _ := setupService()
	jwtSecret := "test-secret-key-at-least-32b"

	result, err := svc.Register(context.Background(), commands.RegisterCommand{
		Email:    "test@test.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	claims, err := auth.ValidateToken(jwtSecret, result.Token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.UserID != result.User.ID {
		t.Errorf("UserID in token = %q, want %q", claims.UserID, result.User.ID)
	}
}
