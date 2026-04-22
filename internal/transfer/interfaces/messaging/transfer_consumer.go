package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/felipersas/payflow/internal/transfer/application/services"
	"github.com/felipersas/payflow/pkg/messaging"
)

// TransferConsumer ouve eventos do account-service e delega ao service.
// Camada fina: só desserializa a mensagem e chama o método correto do service.
type TransferConsumer struct {
	service  *services.TransferService
	consumer *messaging.Consumer
	logger   *slog.Logger
}

func NewTransferConsumer(
	service *services.TransferService,
	consumer *messaging.Consumer,
	logger *slog.Logger,
) *TransferConsumer {
	return &TransferConsumer{
		service:  service,
		consumer: consumer,
		logger:   logger,
	}
}

// Start registra os handlers para eventos do account-service.
func (c *TransferConsumer) Start(ctx context.Context) error {
	if err := c.consumer.Consume(ctx,
		"transfer-service.account.debited",
		"account.debited",
		c.handleAccountDebited,
	); err != nil {
		return fmt.Errorf("registering debited handler: %w", err)
	}

	if err := c.consumer.Consume(ctx,
		"transfer-service.account.credited",
		"account.credited",
		c.handleAccountCredited,
	); err != nil {
		return fmt.Errorf("registering credited handler: %w", err)
	}

	if err := c.consumer.Consume(ctx,
		"transfer-service.account.debit.failed",
		"account.debit.failed",
		c.handleDebitFailed,
	); err != nil {
		return fmt.Errorf("registering debit failed handler: %w", err)
	}

	if err := c.consumer.Consume(ctx,
		"transfer-service.account.credit.failed",
		"account.credit.failed",
		c.handleCreditFailed,
	); err != nil {
		return fmt.Errorf("registering credit failed handler: %w", err)
	}

	c.logger.Info("transfer consumer started")
	return nil
}

// accountEventMessage é o formato dos eventos publicados pelo account-service.
type accountEventMessage struct {
	AccountID    string `json:"account_id"`
	Amount       int64  `json:"amount"`
	Reference    string `json:"reference"`
	BalanceAfter int64  `json:"balance_after"`
}

// accountFailedMessage é o formato dos eventos de falha do account-service.
type accountFailedMessage struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
	Reference string `json:"reference"`
	Reason    string `json:"reason"`
}

func (c *TransferConsumer) handleAccountDebited(ctx context.Context, body []byte) error {
	var msg accountEventMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("unmarshaling account debited event: %w", err)
	}

	return c.service.HandleDebitConfirmed(ctx, msg.Reference)
}

func (c *TransferConsumer) handleAccountCredited(ctx context.Context, body []byte) error {
	var msg accountEventMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("unmarshaling account credited event: %w", err)
	}

	return c.service.HandleCreditConfirmed(ctx, msg.Reference)
}

func (c *TransferConsumer) handleDebitFailed(ctx context.Context, body []byte) error {
	var msg accountFailedMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("unmarshaling account debit failed event: %w", err)
	}

	return c.service.HandleDebitFailed(ctx, msg.Reference, msg.Reason)
}

func (c *TransferConsumer) handleCreditFailed(ctx context.Context, body []byte) error {
	var msg accountFailedMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("unmarshaling account credit failed event: %w", err)
	}

	return c.service.HandleCreditFailed(ctx, msg.Reference, msg.Reason)
}
