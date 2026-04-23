package messaging

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestResilientPublisher_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPub := NewMockMessagePublisher(ctrl)
	pub := NewResilientPublisher(mockPub, newLogger())

	ctx := context.Background()
	mockPub.EXPECT().Publish(ctx, "test.key", "event").Return(nil)

	err := pub.Publish(ctx, "test.key", "event")

	assert.NoError(t, err, "Publish failed")
}

func TestResilientPublisher_Failure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockErr := errors.New("broker unavailable")
	mockPub := NewMockMessagePublisher(ctrl)
	pub := NewResilientPublisher(mockPub, newLogger())

	ctx := context.Background()
	mockPub.EXPECT().Publish(ctx, "test.key", "event").Return(mockErr)

	err := pub.Publish(ctx, "test.key", "event")

	assert.Error(t, err, "expected error")
	assert.Contains(t, err.Error(), "circuit breaker", "error should mention 'circuit breaker'")
}

func TestResilientPublisher_CircuitBreaker(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockErr := errors.New("broker unavailable")
	mockPub := NewMockMessagePublisher(ctrl)
	pub := NewResilientPublisher(mockPub, newLogger())

	ctx := context.Background()

	// First 5 calls should fail and increment consecutive failures
	for i := 0; i < 5; i++ {
		mockPub.EXPECT().Publish(ctx, "test.key", "event").Return(mockErr)
		err := pub.Publish(ctx, "test.key", "event")
		assert.Error(t, err, "call %d: expected error", i+1)
		assert.Contains(t, err.Error(), "circuit breaker", "call %d: error should mention 'circuit breaker'", i+1)
	}

	// 6th call should get gobreaker.ErrOpenState wrapped
	mockPub.EXPECT().Publish(ctx, "test.key", "event").Return(mockErr)
	err := pub.Publish(ctx, "test.key", "event")
	assert.Error(t, err, "6th call: expected error")
	assert.Contains(t, err.Error(), "circuit breaker", "6th call error should mention 'circuit breaker'")
}

func TestResilientPublisher_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPub := NewMockMessagePublisher(ctrl)
	pub := NewResilientPublisher(mockPub, newLogger())

	mockPub.EXPECT().Close().Return(nil)

	err := pub.Close()
	assert.NoError(t, err, "Close failed")
}
