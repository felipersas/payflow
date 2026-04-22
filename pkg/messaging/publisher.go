package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

const exchangeName = "payflow.events"

// Publisher genérico para publicar eventos no RabbitMQ.
type Publisher struct {
	channel *amqp.Channel
	logger  *slog.Logger
}

func NewPublisher(conn *amqp.Connection, logger *slog.Logger) (*Publisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("opening channel: %w", err)
	}

	// Declara exchange topic para roteamento por tipo de evento
	err = ch.ExchangeDeclare(
		exchangeName,
		"topic",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("declaring exchange: %w", err)
	}

	return &Publisher{
		channel: ch,
		logger:  logger,
	}, nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, event any) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	err = p.channel.PublishWithContext(ctx,
		exchangeName,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publishing event: %w", err)
	}

	p.logger.Info("event published", "routing_key", routingKey, "event_type", routingKey)
	return nil
}

func (p *Publisher) Close() error {
	return p.channel.Close()
}
