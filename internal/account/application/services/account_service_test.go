package services

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/felipersas/payflow/internal/account/application/commands"
	"github.com/felipersas/payflow/internal/account/application/queries"
	"github.com/felipersas/payflow/internal/account/domain/entities"
	"github.com/felipersas/payflow/internal/account/domain/repositories"
)

// mockRepo implementa repositories.AccountRepository para testes.
// Não usa banco real — valida a lógica de aplicação isoladamente.
type mockRepo struct {
	accounts     map[string]*entities.Account
	transactions map[string]*repositories.Transaction
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		accounts:     make(map[string]*entities.Account),
		transactions: make(map[string]*repositories.Transaction),
	}
}

func (m *mockRepo) Create(_ context.Context, account *entities.Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, id string) (*entities.Account, error) {
	a, ok := m.accounts[id]
	if !ok {
		return nil, fmt.Errorf("account %s not found", id)
	}
	return a, nil
}

func (m *mockRepo) GetByUserID(_ context.Context, userID string) (*entities.Account, error) {
	for _, a := range m.accounts {
		if a.UserID == userID {
			return a, nil
		}
	}
	return nil, fmt.Errorf("account for user %s not found", userID)
}

func (m *mockRepo) Update(_ context.Context, account *entities.Account) error {
	m.accounts[account.ID] = account
	return nil
}

func (m *mockRepo) GetByReference(_ context.Context, reference string) (*repositories.Transaction, error) {
	tx, ok := m.transactions[reference]
	if !ok {
		return nil, nil
	}
	return tx, nil
}

func (m *mockRepo) SaveTransaction(_ context.Context, tx *repositories.Transaction) error {
	m.transactions[tx.Reference] = tx
	return nil
}

func setupService() (*AccountService, *mockRepo) {
	repo := newMockRepo()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	// Publisher nil — testamos lógica, não mensageria
	return &AccountService{
		repo:   repo,
		logger: logger,
	}, repo
}

func TestCreateAccount(t *testing.T) {
	svc, _ := setupService()

	account, err := svc.CreateAccount(context.Background(), commands.CreateAccountCommand{
		UserID:   "user-1",
		Currency: "BRL",
	})
	if err != nil {
		t.Fatalf("CreateAccount() error = %v", err)
	}
	if account.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", account.UserID, "user-1")
	}
	if account.Currency != "BRL" {
		t.Errorf("Currency = %q, want %q", account.Currency, "BRL")
	}
	if account.Balance != 0 {
		t.Errorf("Balance = %d, want 0", account.Balance)
	}
}

func TestCreditAccount(t *testing.T) {
	svc, repo := setupService()

	account, _ := svc.CreateAccount(context.Background(), commands.CreateAccountCommand{
		UserID: "user-1", Currency: "BRL",
	})

	result, err := svc.CreditAccount(context.Background(), commands.CreditAccountCommand{
		AccountID: account.ID,
		Amount:    5000, // R$ 50,00
		Reference: "ref-credit-1",
	})
	if err != nil {
		t.Fatalf("CreditAccount() error = %v", err)
	}
	if result.Balance != 5000 {
		t.Errorf("Balance after credit = %d, want 5000", result.Balance)
	}

	// Verifica que a transação foi salva
	tx, _ := repo.GetByReference(context.Background(), "ref-credit-1")
	if tx == nil {
		t.Error("transaction not saved")
	}
}

func TestDebitAccount(t *testing.T) {
	svc, _ := setupService()

	account, _ := svc.CreateAccount(context.Background(), commands.CreateAccountCommand{
		UserID: "user-1", Currency: "BRL",
	})
	_, _ = svc.CreditAccount(context.Background(), commands.CreditAccountCommand{
		AccountID: account.ID, Amount: 10000, Reference: "ref-credit-1",
	})

	result, err := svc.DebitAccount(context.Background(), commands.DebitAccountCommand{
		AccountID: account.ID,
		Amount:    3000,
		Reference: "ref-debit-1",
	})
	if err != nil {
		t.Fatalf("DebitAccount() error = %v", err)
	}
	if result.Balance != 7000 {
		t.Errorf("Balance after debit = %d, want 7000", result.Balance)
	}
}

func TestDebitInsufficientBalance(t *testing.T) {
	svc, _ := setupService()

	account, _ := svc.CreateAccount(context.Background(), commands.CreateAccountCommand{
		UserID: "user-1", Currency: "BRL",
	})

	_, err := svc.DebitAccount(context.Background(), commands.DebitAccountCommand{
		AccountID: account.ID,
		Amount:    100,
		Reference: "ref-debit-fail",
	})
	if err == nil {
		t.Error("expected error for insufficient balance")
	}
}

func TestIdempotency_DuplicateCredit(t *testing.T) {
	svc, _ := setupService()

	account, _ := svc.CreateAccount(context.Background(), commands.CreateAccountCommand{
		UserID: "user-1", Currency: "BRL",
	})

	// Primeiro crédito
	result1, _ := svc.CreditAccount(context.Background(), commands.CreditAccountCommand{
		AccountID: account.ID, Amount: 5000, Reference: "ref-dup",
	})

	// Segundo crédito com mesma referência — deve ser ignorado
	result2, err := svc.CreditAccount(context.Background(), commands.CreditAccountCommand{
		AccountID: account.ID, Amount: 5000, Reference: "ref-dup",
	})
	if err != nil {
		t.Fatalf("duplicate credit should not error, got: %v", err)
	}
	if result2.Balance != result1.Balance {
		t.Errorf("Balance after duplicate = %d, want %d (same as first)", result2.Balance, result1.Balance)
	}
}

func TestIdempotency_DuplicateDebit(t *testing.T) {
	svc, _ := setupService()

	account, _ := svc.CreateAccount(context.Background(), commands.CreateAccountCommand{
		UserID: "user-1", Currency: "BRL",
	})
	_, _ = svc.CreditAccount(context.Background(), commands.CreditAccountCommand{
		AccountID: account.ID, Amount: 10000, Reference: "ref-credit",
	})

	// Primeiro débito
	result1, _ := svc.DebitAccount(context.Background(), commands.DebitAccountCommand{
		AccountID: account.ID, Amount: 3000, Reference: "ref-dup-debit",
	})

	// Segundo débito com mesma referência — deve ser ignorado
	result2, err := svc.DebitAccount(context.Background(), commands.DebitAccountCommand{
		AccountID: account.ID, Amount: 3000, Reference: "ref-dup-debit",
	})
	if err != nil {
		t.Fatalf("duplicate debit should not error, got: %v", err)
	}
	if result2.Balance != result1.Balance {
		t.Errorf("Balance after duplicate = %d, want %d", result2.Balance, result1.Balance)
	}
}

func TestGetBalance(t *testing.T) {
	svc, _ := setupService()

	account, _ := svc.CreateAccount(context.Background(), commands.CreateAccountCommand{
		UserID: "user-1", Currency: "BRL",
	})
	_, _ = svc.CreditAccount(context.Background(), commands.CreditAccountCommand{
		AccountID: account.ID, Amount: 7500, Reference: "ref-1",
	})

	result, err := svc.GetBalance(context.Background(), queries.GetBalanceQuery{
		AccountID: account.ID,
	})
	if err != nil {
		t.Fatalf("GetBalance() error = %v", err)
	}
	if result.Balance != 7500 {
		t.Errorf("Balance = %d, want 7500", result.Balance)
	}
	if result.Currency != "BRL" {
		t.Errorf("Currency = %q, want %q", result.Currency, "BRL")
	}
}

func TestGetBalanceNotFound(t *testing.T) {
	svc, _ := setupService()

	_, err := svc.GetBalance(context.Background(), queries.GetBalanceQuery{
		AccountID: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent account")
	}
}
