package messaging

import "context"

//go:generate mockgen -source=interfaces.go -destination=mock_interfaces.go -package=messaging

// MessagePublisher is the interface for publishing events.
// Services should depend on this interface, not the concrete Publisher.
type MessagePublisher interface {
	Publish(ctx context.Context, routingKey string, event any) error
	Close() error
}
