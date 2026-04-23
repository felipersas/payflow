package entities

import (
	"testing"
	"time"
)

func TestNewUser(t *testing.T) {
	tests := []struct {
		name         string
		email        string
		passwordHash string
		wantErr      bool
	}{
		{
			name:         "valid user",
			email:        "test@test.com",
			passwordHash: "$2a$10$abc",
			wantErr:      false,
		},
		{
			name:         "empty email",
			email:        "",
			passwordHash: "$2a$10$abc",
			wantErr:      true,
		},
		{
			name:         "empty passwordHash",
			email:        "test@test.com",
			passwordHash: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now().UTC()
			user, err := NewUser(tt.email, tt.passwordHash)
			after := time.Now().UTC()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user.ID == "" {
				t.Error("ID should not be empty")
			}
			if user.Email != tt.email {
				t.Errorf("Email = %q, want %q", user.Email, tt.email)
			}
			if user.PasswordHash != tt.passwordHash {
				t.Errorf("PasswordHash = %q, want %q", user.PasswordHash, tt.passwordHash)
			}
			if user.CreatedAt.Before(before) || user.CreatedAt.After(after) {
				t.Error("CreatedAt should be close to now")
			}
			if user.UpdatedAt.Before(before) || user.UpdatedAt.After(after) {
				t.Error("UpdatedAt should be close to now")
			}
		})
	}
}

func TestUser_TimestampsEqualInitially(t *testing.T) {
	user, _ := NewUser("test@test.com", "$2a$10$abc")

	if user.CreatedAt != user.UpdatedAt {
		t.Error("CreatedAt and UpdatedAt should be equal initially")
	}
}
