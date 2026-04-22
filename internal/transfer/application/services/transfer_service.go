package services

import (
	"context"
	"log/slog"

	"github.com/felipersas/payflow/internal/transfer/application/commands"
	"github.com/felipersas/payflow/internal/transfer/application/queries"
	"github.com/felipersas/payflow/internal/transfer/domain/repositories"
	"github.com/felipersas/payflow/pkg/events"
	"github.com/felipersas/payflow/pkg/messaging"
)

type TransferService struct {
	repo      repositories.TransferRepository
	publisher *messaging.Publisher
	logger    *slog.Logger
}

func NewTransferService(
	repo repositories.TransferRepository,
	publisher *messaging.Publisher,
	logger *slog.Logger,
) *TransferService {
	return &TransferService{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
	}
}

// CreateTransfer cria uma transferência e publica evento TransferCreated.
func (s *TransferService) CreateTransfer(ctx context.Context, cmd commands.CreateTransferCommand) (*queries.TransferResult, error) {
	// Lógica de criação de transferência (validação, persistência, etc.)
	// ...

	// Exemplo de publicação de evento
	event := &events.TransferOcurred{}
	s.publishEvent(ctx, "TransferCreated", event)

	s.logger.Info("transfer created", "from_account_id", cmd.FromAccountID, "to_account_id", cmd.ToAccountID)
	return &queries.TransferResult{
		// Preencher resultado
	}, nil
}

func (s *TransferService) publishEvent(ctx context.Context, routingKey string, event any) {
	if s.publisher == nil {
		return
	}
	if err := s.publisher.Publish(ctx, routingKey, event); err != nil {
		s.logger.Error("failed to publish event", "routing_key", routingKey, "error", err)
	}
}
