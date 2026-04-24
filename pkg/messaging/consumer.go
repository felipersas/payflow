package messaging

import (
	"context"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/felipersas/payflow/pkg/telemetry"
)

// Handler é a função que processa cada mensagem recebida.
// Retorna error para nack (vai para DLQ).
type Handler func(ctx context.Context, body []byte) error

// Consumer genérico para consumir filas do RabbitMQ com DLQ.
type Consumer struct {
	channel *amqp.Channel
	logger  *slog.Logger
}

func NewConsumer(conn *amqp.Connection, logger *slog.Logger) (*Consumer, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("opening channel: %w", err)
	}

	// QoS: prefetch de 10 mensagens por worker
	err = ch.Qos(10, 0, false)
	if err != nil {
		return nil, fmt.Errorf("setting QoS: %w", err)
	}

	return &Consumer{
		channel: ch,
		logger:  logger,
	}, nil
}

// Consume declara fila + DLQ, faz bind na exchange e começa a consumir.
// queueName: nome da fila (ex: "account-service.credit.cmd")
// routingKey: chave de roteamento (ex: "account.credit.cmd")
func (c *Consumer) Consume(ctx context.Context, queueName, routingKey string, handler Handler) error {
	// Declara DLQ (Dead Letter Queue)
	dlqName := queueName + ".dlq"
	_, err := c.channel.QueueDeclare(dlqName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declaring DLQ %s: %w", dlqName, err)
	}

	// Declara fila principal com DLQ configurada
	queue, err := c.channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-deleted
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": dlqName,
		},
	)
	if err != nil {
		return fmt.Errorf("declaring queue %s: %w", queueName, err)
	}

	// Bind na exchange
	err = c.channel.QueueBind(queue.Name, routingKey, exchangeName, false, nil)
	if err != nil {
		return fmt.Errorf("binding queue %s to %s: %w", queueName, routingKey, err)
	}

	deliveries, err := c.channel.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consuming from %s: %w", queueName, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.logger.Info("consumer stopped", "queue", queueName)
				return
			case d, ok := <-deliveries:
				if !ok {
					c.logger.Info("deliveries channel closed", "queue", queueName)
					return
				}
				if err := handler(ctx, d.Body); err != nil {
					c.logger.Error("handler error, nacking", "queue", queueName, "error", err)
					telemetry.RabbitMQConsumeTotal.WithLabelValues(queueName, "error").Inc()
					d.Nack(false, false) // requeue=false → vai para DLQ
					continue
				}
				telemetry.RabbitMQConsumeTotal.WithLabelValues(queueName, "success").Inc()
				d.Ack(false)
			}
		}
	}()

	c.logger.Info("consumer started", "queue", queueName, "routing_key", routingKey)
	return nil
}

func (c *Consumer) Close() error {
	return c.channel.Close()
}
