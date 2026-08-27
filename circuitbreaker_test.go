package httpclient

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/sony/gobreaker"
)

func TestNewCircuitBreakerNilConfig(t *testing.T) {
	cb := NewCircuitBreaker(nil, nil)
	if cb == nil {
		t.Fatal("expected non-nil breaker")
	}
	result, err := cb.Execute(func() (any, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %v", result)
	}
}

func TestNewCircuitBreakerWithConfigAndDefaultReadyToTrip(t *testing.T) {
	cfg := &CircuitBreakerConfig{
		Name:        "custom",
		MaxRequests: 3,
		Interval:    time.Minute,
		Timeout:     2 * time.Minute,
	}
	cb := NewCircuitBreaker(cfg, nil)
	if cb == nil {
		t.Fatal("expected non-nil breaker")
	}
	if cb.Name() != "custom" {
		t.Fatalf("expected name 'custom', got %q", cb.Name())
	}
	result, err := cb.Execute(func() (any, error) {
		return &http.Response{StatusCode: http.StatusOK}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.(*http.Response); !ok {
		t.Fatal("expected *http.Response result")
	}
}

func TestNewCircuitBreakerOnStateChange(t *testing.T) {
	var mu sync.Mutex
	var transitions []gobreaker.State
	onChange := func(name string, from, to gobreaker.State) {
		mu.Lock()
		transitions = append(transitions, from, to)
		mu.Unlock()
	}

	cfg := &CircuitBreakerConfig{Name: "trip", Counts: 1, MaxRequests: 1, Timeout: time.Hour}
	cb := NewCircuitBreaker(cfg, onChange)

	_, err := cb.Execute(func() (any, error) {
		return nil, errors.New("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = cb.Execute(func() (any, error) {
		return "never", nil
	})
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("expected open state, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(transitions) < 2 {
		t.Fatalf("expected state change notification, got %v", transitions)
	}
	if transitions[0] != gobreaker.StateClosed || transitions[1] != gobreaker.StateOpen {
		t.Fatalf("expected closed -> open transition, got %v", transitions)
	}
}

func TestCircuitBreakerWithContext(t *testing.T) {
	cfg := &CircuitBreakerConfig{Name: "ctx", Counts: 10}
	cb := NewCircuitBreaker(cfg, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := cb.Execute(func() (any, error) {
		return ctx.Err(), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil ctx error result")
	}
}
