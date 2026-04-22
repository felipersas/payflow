package messaging

import "context"

// MessagePublisher is the interface for publishing events.
// Services should depend on this interface, not the concrete Publisher.
type MessagePublisher interface {
	Publish(ctx context.Context, routingKey string, event any) error
	Close() error
}
