package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				assert.Error(t, err, "expected error")
				return
			}
			require.NoError(t, err, "unexpected error")
			assert.NotEmpty(t, user.ID)
			assert.Equal(t, tt.email, user.Email)
			assert.Equal(t, tt.passwordHash, user.PasswordHash)
			assert.True(t, user.CreatedAt.After(before) || user.CreatedAt.Before(after), "CreatedAt should be close to now")
			assert.True(t, user.UpdatedAt.After(before) || user.UpdatedAt.Before(after), "UpdatedAt should be close to now")
		})
	}
}

func TestUser_TimestampsEqualInitially(t *testing.T) {
	user, _ := NewUser("test@test.com", "$2a$10$abc")

	assert.Equal(t, user.CreatedAt, user.UpdatedAt, "CreatedAt and UpdatedAt should be equal initially")
}
