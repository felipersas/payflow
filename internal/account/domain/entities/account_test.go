package entities

import (
	"testing"
)

func TestNewAccount(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		currency string
		wantErr  bool
	}{
		{"valid account", "user-123", "BRL", false},
		{"empty user_id", "", "BRL", true},
		{"empty currency", "user-123", "", true},
		{"invalid currency length", "user-123", "REAL", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, err := NewAccount(tt.userID, tt.currency)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if account.UserID != tt.userID {
				t.Errorf("UserID = %q, want %q", account.UserID, tt.userID)
			}
			if account.Currency != tt.currency {
				t.Errorf("Currency = %q, want %q", account.Currency, tt.currency)
			}
			if account.Balance != 0 {
				t.Errorf("Balance = %d, want 0", account.Balance)
			}
			if !account.IsActive {
				t.Error("IsActive = false, want true")
			}
			if account.ID == "" {
				t.Error("ID should not be empty")
			}
		})
	}
}

func TestAccount_Credit(t *testing.T) {
	account, _ := NewAccount("user-1", "BRL")

	// Crédito normal
	event, err := account.Credit(10000, "ref-1") // R$ 100,00
	if err != nil {
		t.Fatalf("Credit() error = %v", err)
	}
	if account.Balance != 10000 {
		t.Errorf("Balance after credit = %d, want 10000", account.Balance)
	}
	if event.Amount != 10000 {
		t.Errorf("event Amount = %d, want 10000", event.Amount)
	}
	if event.BalanceAfter != 10000 {
		t.Errorf("event BalanceAfter = %d, want 10000", event.BalanceAfter)
	}

	// Crédito com amount zero
	_, err = account.Credit(0, "ref-2")
	if err == nil {
		t.Error("expected error for zero amount")
	}

	// Crédito com amount negativo
	_, err = account.Credit(-100, "ref-3")
	if err == nil {
		t.Error("expected error for negative amount")
	}

	// Crédito sem referência
	_, err = account.Credit(100, "")
	if err == nil {
		t.Error("expected error for empty reference")
	}
}

func TestAccount_Debit(t *testing.T) {
	account, _ := NewAccount("user-1", "BRL")
	_, _ = account.Credit(10000, "ref-credit") // R$ 100,00

	// Débito normal
	event, err := account.Debit(3000, "ref-debit") // R$ 30,00
	if err != nil {
		t.Fatalf("Debit() error = %v", err)
	}
	if account.Balance != 7000 {
		t.Errorf("Balance after debit = %d, want 7000", account.Balance)
	}
	if event.Amount != 3000 {
		t.Errorf("event Amount = %d, want 3000", event.Amount)
	}
	if event.BalanceAfter != 7000 {
		t.Errorf("event BalanceAfter = %d, want 7000", event.BalanceAfter)
	}

	// Débito com saldo insuficiente
	_, err = account.Debit(8000, "ref-debit-2")
	if err == nil {
		t.Error("expected error for insufficient balance")
	}

	// Débito com amount zero
	_, err = account.Debit(0, "ref-debit-3")
	if err == nil {
		t.Error("expected error for zero amount")
	}

	// Débito sem referência
	_, err = account.Debit(100, "")
	if err == nil {
		t.Error("expected error for empty reference")
	}
}

func TestAccount_DebitBlocked(t *testing.T) {
	account, _ := NewAccount("user-1", "BRL")
	_, _ = account.Credit(10000, "ref-1")

	account.Block("fraud suspected")

	_, err := account.Debit(1000, "ref-blocked")
	if err == nil {
		t.Error("expected error when debiting blocked account")
	}
}

func TestAccount_CreditBlocked(t *testing.T) {
	account, _ := NewAccount("user-1", "BRL")

	account.Block("admin action")

	_, err := account.Credit(1000, "ref-blocked")
	if err == nil {
		t.Error("expected error when crediting blocked account")
	}
}

func TestAccount_Block(t *testing.T) {
	account, _ := NewAccount("user-1", "BRL")

	event := account.Block("suspicious activity")
	if account.IsActive {
		t.Error("IsActive should be false after block")
	}
	if event.Reason != "suspicious activity" {
		t.Errorf("event Reason = %q, want %q", event.Reason, "suspicious activity")
	}
}

func TestAccount_Unblock(t *testing.T) {
	account, _ := NewAccount("user-1", "BRL")
	account.Block("test")
	account.Unblock()

	if !account.IsActive {
		t.Error("IsActive should be true after unblock")
	}
}

func TestAccount_VersionIncrements(t *testing.T) {
	account, _ := NewAccount("user-1", "BRL")
	if account.Version != 1 {
		t.Errorf("initial Version = %d, want 1", account.Version)
	}

	_, _ = account.Credit(1000, "ref-1")
	if account.Version != 2 {
		t.Errorf("Version after credit = %d, want 2", account.Version)
	}

	_, _ = account.Debit(500, "ref-2")
	if account.Version != 3 {
		t.Errorf("Version after debit = %d, want 3", account.Version)
	}
}
