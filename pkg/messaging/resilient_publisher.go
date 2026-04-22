package messaging

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/sony/gobreaker"
)

// Compile-time check that Publisher implements MessagePublisher.
var _ MessagePublisher = (*Publisher)(nil)

// ResilientPublisher wraps a MessagePublisher with circuit breaker protection.
type ResilientPublisher struct {
	inner  MessagePublisher
	cb     *gobreaker.CircuitBreaker
	logger *slog.Logger
}

// NewResilientPublisher creates a circuit-breaker-backed publisher.
// Settings: 5 consecutive failures opens, 30s timeout, 3 half-open requests.
func NewResilientPublisher(inner MessagePublisher, logger *slog.Logger) *ResilientPublisher {
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "rabbitmq-publisher",
		MaxRequests: 3,
		Interval:    0,
		Timeout:     30,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Warn("circuit breaker state changed",
				"name", name,
				"from", from.String(),
				"to", to.String(),
			)
		},
	})

	return &ResilientPublisher{
		inner:  inner,
		cb:     cb,
		logger: logger,
	}
}

func (p *ResilientPublisher) Publish(ctx context.Context, routingKey string, event any) error {
	_, err := p.cb.Execute(func() (interface{}, error) {
		return nil, p.inner.Publish(ctx, routingKey, event)
	})
	if err != nil {
		return fmt.Errorf("circuit breaker: %w", err)
	}
	return nil
}

func (p *ResilientPublisher) Close() error {
	return p.inner.Close()
}
