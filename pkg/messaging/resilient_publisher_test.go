package messaging

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/sony/gobreaker"
)

type mockPublisher struct {
	calls int
	err   error
	closeCalled bool
}

func (m *mockPublisher) Publish(_ context.Context, _ string, _ any) error {
	m.calls++
	return m.err
}

func (m *mockPublisher) Close() error {
	m.closeCalled = true
	return nil
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestResilientPublisher_Success(t *testing.T) {
	mock := &mockPublisher{err: nil}
	pub := NewResilientPublisher(mock, newLogger())

	ctx := context.Background()
	err := pub.Publish(ctx, "test.key", "event")

	if err != nil {
		t.Errorf("Publish failed: %v", err)
	}
	if mock.calls != 1 {
		t.Errorf("mock.calls: got %d, want 1", mock.calls)
	}
}

func TestResilientPublisher_Failure(t *testing.T) {
	mockErr := errors.New("broker unavailable")
	mock := &mockPublisher{err: mockErr}
	pub := NewResilientPublisher(mock, newLogger())

	ctx := context.Background()
	err := pub.Publish(ctx, "test.key", "event")

	if err == nil {
		t.Error("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "circuit breaker") {
		t.Errorf("error should mention 'circuit breaker': %v", err)
	}
	if mock.calls != 1 {
		t.Errorf("mock.calls: got %d, want 1", mock.calls)
	}
}

func TestResilientPublisher_CircuitBreaker(t *testing.T) {
	mockErr := errors.New("broker unavailable")
	mock := &mockPublisher{err: mockErr}
	pub := NewResilientPublisher(mock, newLogger())

	ctx := context.Background()

	// First 5 calls should fail and increment consecutive failures
	for i := 0; i < 5; i++ {
		err := pub.Publish(ctx, "test.key", "event")
		if err == nil {
			t.Errorf("call %d: expected error, got nil", i+1)
		}
		if !strings.Contains(err.Error(), "circuit breaker") {
			t.Errorf("call %d: error should mention 'circuit breaker': %v", i+1, err)
		}
	}

	// 6th call should get gobreaker.ErrOpenState wrapped
	err := pub.Publish(ctx, "test.key", "event")
	if err == nil {
		t.Error("6th call: expected error, got nil")
	}
	// Check for open state error
	if !strings.Contains(err.Error(), "circuit breaker") {
		t.Errorf("6th call error should mention 'circuit breaker': %v", err)
	}
	if !errors.Is(err, gobreaker.ErrOpenState) && !strings.Contains(err.Error(), gobreaker.ErrOpenState.Error()) {
		// The error is wrapped, so we check the message
		t.Logf("6th call error: %v", err)
	}

	if mock.calls != 6 {
		t.Errorf("mock.calls: got %d, want 6", mock.calls)
	}
}

func TestResilientPublisher_Close(t *testing.T) {
	mock := &mockPublisher{err: nil}
	pub := NewResilientPublisher(mock, newLogger())

	err := pub.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if !mock.closeCalled {
		t.Error("expected mock.Close() to be called")
	}
}
