package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/felipersas/payflow/internal/transfer/application/commands"
	"github.com/felipersas/payflow/internal/transfer/application/queries"
	"github.com/felipersas/payflow/internal/transfer/domain/entities"
	"github.com/felipersas/payflow/internal/transfer/domain/repositories"
	apperrors "github.com/felipersas/payflow/pkg/errors"
	"github.com/felipersas/payflow/pkg/messaging"
)

type TransferService struct {
	repo      repositories.TransferRepository
	publisher messaging.MessagePublisher
	logger    *slog.Logger
}

func NewTransferService(
	repo repositories.TransferRepository,
	publisher messaging.MessagePublisher,
	logger *slog.Logger,
) *TransferService {
	return &TransferService{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
	}
}

// accountCommand é o formato de mensagem que o account-service espera
// para comandos de crédito e débito.
type accountCommand struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
	Reference string `json:"reference"` // transfer ID para correlação do saga
}

// CreateTransfer cria uma transferência pendente e dispara o passo 1 do saga:
// envia o comando de débito para o account-service.
func (s *TransferService) CreateTransfer(ctx context.Context, cmd commands.CreateTransferCommand) (*queries.TransferResult, error) {
	transfer, err := entities.NewTransfer(cmd.FromAccountID, cmd.ToAccountID, cmd.Amount, cmd.Currency)
	if err != nil {
		return nil, fmt.Errorf("creating transfer entity: %w", err)
	}

	if err := s.repo.Create(ctx, transfer); err != nil {
		return nil, fmt.Errorf("persisting transfer: %w", err)
	}

	// Saga passo 1: solicita débito na conta de origem
	if err := s.sendDebitCommand(ctx, transfer); err != nil {
		return nil, err
	}

	s.logger.Info("transfer created, debit command sent",
		"transfer_id", transfer.ID,
		"from_account_id", transfer.FromAccountID,
		"to_account_id", transfer.ToAccountID,
		"amount", transfer.Amount,
	)

	return &queries.TransferResult{
		TransferID:    transfer.ID,
		FromAccountID: transfer.FromAccountID,
		ToAccountID:   transfer.ToAccountID,
		Amount:        transfer.Amount,
		Currency:      transfer.Currency,
		Status:        transfer.Status,
	}, nil
}

// HandleDebitConfirmed o saga quando o débito é confirmado:
// atualiza status para "processing" e envia comando de crédito para a conta de destino.
func (s *TransferService) HandleDebitConfirmed(ctx context.Context, transferID string) error {
	transfer, err := s.repo.GetByID(ctx, transferID)
	if err != nil {
		return fmt.Errorf("finding transfer %s: %w", transferID, err)
	}
	if transfer == nil {
		return apperrors.NotFound("transfer %s not found", transferID)
	}
	if !transfer.IsPending() {
		s.logger.Warn("transfer already processed, skipping debit confirmation",
			"transfer_id", transfer.ID,
			"status", transfer.Status,
		)
		return nil
	}

	// Atualiza status intermediário
	if err := s.repo.UpdateStatus(ctx, transfer.ID, string(entities.TransferProcessing)); err != nil {
		return fmt.Errorf("updating transfer %s to processing: %w", transfer.ID, err)
	}

	if err := s.sendCreditCommand(ctx, transfer); err != nil {
		// Falha ao enviar crédito: marca como failed
		if failErr := s.repo.UpdateStatus(ctx, transfer.ID, string(entities.TransferFailed)); failErr != nil {
			s.logger.Error("failed to mark transfer as failed after credit command error",
				"transfer_id", transfer.ID,
				"error", failErr,
			)
		}
		return fmt.Errorf("sending credit command for transfer %s: %w", transfer.ID, err)
	}

	s.logger.Info("debit confirmed, credit command sent",
		"transfer_id", transfer.ID,
		"to_account_id", transfer.ToAccountID,
	)
	return nil
}

// HandleCreditConfirmed avança o saga quando o crédito é confirmado:
// marca a transferência como concluída e publica evento de domínio.
func (s *TransferService) HandleCreditConfirmed(ctx context.Context, transferID string) error {
	transfer, err := s.repo.GetByID(ctx, transferID)
	if err != nil {
		return fmt.Errorf("finding transfer %s: %w", transferID, err)
	}
	if transfer == nil {
		return apperrors.NotFound("transfer %s not found", transferID)
	}
	if transfer.IsCompleted() {
		s.logger.Warn("transfer already completed, skipping credit confirmation",
			"transfer_id", transfer.ID,
			"status", transfer.Status,
		)
		return nil
	}

	event, err := transfer.MarkCompleted()
	if err != nil {
		return fmt.Errorf("marking transfer %s completed: %w", transfer.ID, err)
	}

	if err := s.repo.UpdateStatus(ctx, transfer.ID, string(transfer.Status)); err != nil {
		return fmt.Errorf("updating transfer %s status: %w", transfer.ID, err)
	}

	if err := s.publisher.Publish(ctx, "transfer.completed", event); err != nil {
		s.logger.Error("failed to publish transfer completed event",
			"transfer_id", transfer.ID,
			"error", err,
		)
	}

	s.logger.Info("transfer completed",
		"transfer_id", transfer.ID,
		"from_account_id", transfer.FromAccountID,
		"to_account_id", transfer.ToAccountID,
		"amount", transfer.Amount,
	)
	return nil
}

// GetTransfer busca uma transferência pelo ID.
func (s *TransferService) GetTransfer(ctx context.Context, id string) (*queries.TransferResult, error) {
	transfer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding transfer %s: %w", id, err)
	}
	if transfer == nil {
		return nil, apperrors.NotFound("transfer %s not found", id)
	}

	return &queries.TransferResult{
		TransferID:    transfer.ID,
		FromAccountID: transfer.FromAccountID,
		ToAccountID:   transfer.ToAccountID,
		Amount:        transfer.Amount,
		Currency:      transfer.Currency,
		Status:        transfer.Status,
	}, nil
}

// HandleDebitFailed marca a transferência como falha quando o débito é recusado.
func (s *TransferService) HandleDebitFailed(ctx context.Context, transferID string, reason string) error {
	transfer, err := s.repo.GetByID(ctx, transferID)
	if err != nil {
		return fmt.Errorf("finding transfer %s: %w", transferID, err)
	}
	if transfer == nil {
		return fmt.Errorf("transfer %s not found", transferID)
	}
	if !transfer.IsPending() {
		return nil
	}

	event, err := transfer.MarkFailed()
	if err != nil {
		return fmt.Errorf("marking transfer %s failed: %w", transfer.ID, err)
	}

	if err := s.repo.UpdateStatus(ctx, transfer.ID, string(transfer.Status)); err != nil {
		return fmt.Errorf("updating transfer %s status to failed: %w", transfer.ID, err)
	}

	s.logger.Error("transfer failed at debit step",
		"transfer_id", transfer.ID,
		"reason", reason,
	)

	if err := s.publisher.Publish(ctx, "transfer.failed", event); err != nil {
		s.logger.Error("failed to publish transfer failed event",
			"transfer_id", transfer.ID,
			"error", err,
		)
	}

	return nil
}

// HandleCreditFailed marca a transferência como falha quando o crédito é recusado
// e envia comando de compensação para reverter o débito na conta de origem.
func (s *TransferService) HandleCreditFailed(ctx context.Context, transferID string, reason string) error {
	transfer, err := s.repo.GetByID(ctx, transferID)
	if err != nil {
		return fmt.Errorf("finding transfer %s: %w", transferID, err)
	}
	if transfer == nil {
		return fmt.Errorf("transfer %s not found", transferID)
	}
	if transfer.IsCompleted() {
		return nil
	}

	event, err := transfer.MarkFailed()
	if err != nil {
		return fmt.Errorf("marking transfer %s failed: %w", transfer.ID, err)
	}

	if err := s.repo.UpdateStatus(ctx, transfer.ID, string(transfer.Status)); err != nil {
		return fmt.Errorf("updating transfer %s status to failed: %w", transfer.ID, err)
	}

	// Saga compensation: reverse the debit on source account
	compensateCmd := accountCommand{
		AccountID: transfer.FromAccountID,
		Amount:    transfer.Amount,
		Reference: "compensate-" + transfer.ID,
	}
	if err := s.publisher.Publish(ctx, "account.compensate.cmd", compensateCmd); err != nil {
		s.logger.Error("failed to send compensate command",
			"transfer_id", transfer.ID,
			"error", err,
		)
	}

	s.logger.Error("transfer failed at credit step, compensation sent",
		"transfer_id", transfer.ID,
		"reason", reason,
	)

	if err := s.publisher.Publish(ctx, "transfer.failed", event); err != nil {
		s.logger.Error("failed to publish transfer failed event",
			"transfer_id", transfer.ID,
			"error", err,
		)
	}

	return nil
}

func (s *TransferService) sendDebitCommand(ctx context.Context, transfer *entities.Transfer) error {
	debitCmd := accountCommand{
		AccountID: transfer.FromAccountID,
		Amount:    transfer.Amount,
		Reference: "debit-" + transfer.ID,
	}
	if err := s.publisher.Publish(ctx, "account.debit.cmd", debitCmd); err != nil {
		return fmt.Errorf("sending debit command: %w", err)
	}
	return nil
}

func (s *TransferService) sendCreditCommand(ctx context.Context, transfer *entities.Transfer) error {
	creditCmd := accountCommand{
		AccountID: transfer.ToAccountID,
		Amount:    transfer.Amount,
		Reference: "credit-" + transfer.ID,
	}
	if err := s.publisher.Publish(ctx, "account.credit.cmd", creditCmd); err != nil {
		return fmt.Errorf("sending credit command: %w", err)
	}
	return nil
}
