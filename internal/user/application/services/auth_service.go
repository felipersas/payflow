package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/felipersas/payflow/internal/user/application/commands"
	"github.com/felipersas/payflow/internal/user/domain/entities"
	"github.com/felipersas/payflow/internal/user/domain/repositories"
	"github.com/felipersas/payflow/pkg/auth"
	"golang.org/x/crypto/bcrypt"
)

// AuthService orquestra registro e login de usuários.
type AuthService struct {
	userRepo  repositories.UserRepository
	jwtSecret string
	logger    *slog.Logger
}

func NewAuthService(
	userRepo repositories.UserRepository,
	jwtSecret string,
	logger *slog.Logger,
) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

// AuthResult é o retorno de register e login.
type AuthResult struct {
	Token string   `json:"token"`
	User  *UserDTO `json:"user"`
}

// UserDTO é a representação pública do usuário (sem password_hash).
type UserDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (s *AuthService) Register(ctx context.Context, cmd commands.RegisterCommand) (*AuthResult, error) {
	if cmd.Email == "" || cmd.Password == "" {
		return nil, fmt.Errorf("email and password are required")
	}
	if len(cmd.Password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}

	existing, _ := s.userRepo.GetByEmail(ctx, cmd.Email)
	if existing != nil {
		return nil, fmt.Errorf("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cmd.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user, err := entities.NewUser(cmd.Email, string(hash))
	if err != nil {
		return nil, err
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("saving user: %w", err)
	}

	token, err := auth.GenerateToken(s.jwtSecret, user.ID)
	if err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}

	s.logger.Info("user registered", "user_id", user.ID, "email", user.Email)

	return &AuthResult{
		Token: token,
		User:  &UserDTO{ID: user.ID, Email: user.Email},
	}, nil
}

func (s *AuthService) Login(ctx context.Context, cmd commands.LoginCommand) (*AuthResult, error) {
	user, err := s.userRepo.GetByEmail(ctx, cmd.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(cmd.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	token, err := auth.GenerateToken(s.jwtSecret, user.ID)
	if err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}

	s.logger.Info("user logged in", "user_id", user.ID)

	return &AuthResult{
		Token: token,
		User:  &UserDTO{ID: user.ID, Email: user.Email},
	}, nil
}
