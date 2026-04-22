package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/felipersas/payflow/internal/account/application/commands"
	"github.com/felipersas/payflow/internal/account/application/queries"
	"github.com/felipersas/payflow/internal/account/domain/entities"
	"github.com/felipersas/payflow/internal/account/domain/repositories"
	"github.com/felipersas/payflow/pkg/events"
	"github.com/felipersas/payflow/pkg/messaging"
	"github.com/google/uuid"
)

// AccountService orquestra os casos de uso do Account Service.
// Pertence à camada de aplicação: coordena entidades (domínio) e infraestrutura.
type AccountService struct {
	repo      repositories.AccountRepository
	publisher *messaging.Publisher
	logger    *slog.Logger
}

func NewAccountService(
	repo repositories.AccountRepository,
	publisher *messaging.Publisher,
	logger *slog.Logger,
) *AccountService {
	return &AccountService{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
	}
}

// CreateAccount cria uma conta e publica evento AccountCreated.
func (s *AccountService) CreateAccount(ctx context.Context, cmd commands.CreateAccountCommand) (*entities.Account, error) {
	account, err := entities.NewAccount(cmd.UserID, cmd.Currency)
	if err != nil {
		return nil, fmt.Errorf("creating account entity: %w", err)
	}

	if err := s.repo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("saving account: %w", err)
	}

	event := &events.AccountCreated{
		BaseEvent: events.NewBaseEvent(events.AccountCreatedEvent, 1),
		AccountID: account.ID,
		UserID:    account.UserID,
		Currency:  account.Currency,
	}
	s.publishEvent(ctx, events.AccountCreatedEvent, event)

	s.logger.Info("account created", "account_id", account.ID, "user_id", account.UserID)
	return account, nil
}

// DebitAccount debita valor com idempotência baseada em reference.
// Se a referência já foi processada, retorna o resultado anterior.
func (s *AccountService) DebitAccount(ctx context.Context, cmd commands.DebitAccountCommand) (*entities.Account, error) {
	// Idempotência: verifica se já processamos esta referência
	if isDup, err := s.CheckIdempotency(ctx, cmd.Reference, "debit"); err != nil {
		return nil, err
	} else if isDup {
		return s.repo.GetByID(ctx, cmd.AccountID)
	}

	account, err := s.repo.GetByID(ctx, cmd.AccountID)
	if err != nil {
		return nil, fmt.Errorf("finding account %s: %w", cmd.AccountID, err)
	}

	event, err := account.Debit(cmd.Amount, cmd.Reference)
	if err != nil {
		return nil, fmt.Errorf("debiting account: %w", err)
	}

	if err := s.repo.Update(ctx, account); err != nil {
		return nil, fmt.Errorf("updating account: %w", err)
	}

	// Registra transação para idempotência
	tx := &repositories.Transaction{
		ID:           uuid.Must(uuid.NewV7()).String(),
		AccountID:    account.ID,
		Reference:    cmd.Reference,
		Amount:       cmd.Amount,
		Type:         "debit",
		BalanceAfter: account.Balance,
	}
	if err := s.repo.SaveTransaction(ctx, tx); err != nil {
		return nil, fmt.Errorf("saving transaction: %w", err)
	}

	s.publishEvent(ctx, events.AccountDebitedEvent, event)

	s.logger.Info("account debited", "account_id", account.ID, "amount", cmd.Amount, "reference", cmd.Reference)
	return account, nil
}

// CreditAccount credita valor com idempotência baseada em reference.
func (s *AccountService) CreditAccount(ctx context.Context, cmd commands.CreditAccountCommand) (*entities.Account, error) {
	if isDup, err := s.CheckIdempotency(ctx, cmd.Reference, "credit"); err != nil {
		return nil, err
	} else if isDup {
		return s.repo.GetByID(ctx, cmd.AccountID)
	}

	account, err := s.repo.GetByID(ctx, cmd.AccountID)
	if err != nil {
		return nil, fmt.Errorf("finding account %s: %w", cmd.AccountID, err)
	}

	event, err := account.Credit(cmd.Amount, cmd.Reference)
	if err != nil {
		return nil, fmt.Errorf("crediting account: %w", err)
	}

	if err := s.repo.Update(ctx, account); err != nil {
		return nil, fmt.Errorf("updating account: %w", err)
	}

	tx := &repositories.Transaction{
		ID:           uuid.Must(uuid.NewV7()).String(),
		AccountID:    account.ID,
		Reference:    cmd.Reference,
		Amount:       cmd.Amount,
		Type:         "credit",
		BalanceAfter: account.Balance,
	}
	if err := s.repo.SaveTransaction(ctx, tx); err != nil {
		return nil, fmt.Errorf("saving transaction: %w", err)
	}

	s.publishEvent(ctx, events.AccountCreditedEvent, event)

	s.logger.Info("account credited", "account_id", account.ID, "amount", cmd.Amount, "reference", cmd.Reference)
	return account, nil
}

// GetBalance retorna o saldo atual da conta.
func (s *AccountService) GetBalance(ctx context.Context, query queries.GetBalanceQuery) (*queries.BalanceResult, error) {
	account, err := s.repo.GetByID(ctx, query.AccountID)
	if err != nil {
		return nil, fmt.Errorf("finding account %s: %w", query.AccountID, err)
	}

	return &queries.BalanceResult{
		AccountID: account.ID,
		Balance:   account.Balance,
		Currency:  account.Currency,
		IsActive:  account.IsActive,
	}, nil
}

// publishEvent publica um evento se o publisher estiver disponível.
// Em testes, o publisher é nil — o evento é ignorado.
func (s *AccountService) publishEvent(ctx context.Context, routingKey string, event any) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.Publish(ctx, routingKey, event); err != nil {
		s.logger.Error("failed to publish event", "routing_key", routingKey, "error", err)
	}
}

func (s *AccountService) CheckIdempotency(ctx context.Context, reference string, action string) (bool, error) {
	existing, err := s.repo.GetByReference(ctx, reference)
	if err != nil {
		return false, err
	}
	if existing != nil {
		s.logger.Info("duplicate "+action+" ignored (idempotency)", "reference", reference)
		return true, nil
	}
	return false, nil
}
