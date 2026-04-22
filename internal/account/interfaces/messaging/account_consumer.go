package messaging

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/felipersas/payflow/internal/account/application/commands"
	"github.com/felipersas/payflow/internal/account/application/services"
	"github.com/felipersas/payflow/pkg/messaging"
)

// AccountConsumer processa comandos recebidos via RabbitMQ.
// Permite que outros serviços solicitem débito/crédito via mensageria.
type AccountConsumer struct {
	service  *services.AccountService
	consumer *messaging.Consumer
	logger   *slog.Logger
}

func NewAccountConsumer(
	service *services.AccountService,
	consumer *messaging.Consumer,
	logger *slog.Logger,
) *AccountConsumer {
	return &AccountConsumer{
		service:  service,
		consumer: consumer,
		logger:   logger,
	}
}

// Start registra os handlers de crédito e débito no RabbitMQ.
func (c *AccountConsumer) Start(ctx context.Context) error {
	// Consome comandos de crédito
	if err := c.consumer.Consume(ctx,
		"account-service.credit.cmd",
		"account.credit.cmd",
		c.handleCredit,
	); err != nil {
		return err
	}

	// Consome comandos de débito
	if err := c.consumer.Consume(ctx,
		"account-service.debit.cmd",
		"account.debit.cmd",
		c.handleDebit,
	); err != nil {
		return err
	}

	c.logger.Info("account consumer started")
	return nil
}

type creditMessage struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
	Reference string `json:"reference"`
}

func (c *AccountConsumer) handleCredit(ctx context.Context, body []byte) error {
	msg, err := validateBody(body)
	if err != nil {
		return err
	}

	_, err = c.service.CreditAccount(ctx, commands.CreditAccountCommand{
		AccountID: msg.AccountID,
		Amount:    msg.Amount,
		Reference: msg.Reference,
	})
	return err
}

func (c *AccountConsumer) handleDebit(ctx context.Context, body []byte) error {
	msg, err := validateBody(body)
	if err != nil {
		return err
	}

	_, err = c.service.DebitAccount(ctx, commands.DebitAccountCommand{
		AccountID: msg.AccountID,
		Amount:    msg.Amount,
		Reference: msg.Reference,
	})
	return err
}

func validateBody(body []byte) (creditMessage, error) {
	var msg creditMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return creditMessage{}, err
	}
	return msg, nil
}
