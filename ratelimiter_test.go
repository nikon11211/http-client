package httpclient

import (
	"context"
	"testing"
	"time"
)

func TestNewRateLimiterNilConfig(t *testing.T) {
	rl := NewRateLimiter(nil)
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRateLimiterZeroLimit(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{})
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRateLimiterNegativeLimit(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{Limit: -1, Burst: 0})
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRateLimiterDefaultBurst(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{Limit: 100, Burst: 0})
	if rl.limiter == nil {
		t.Fatal("expected a real limiter")
	}
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewRateLimiterNormal(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{Limit: 1000, Burst: 10})
	if rl.limiter == nil {
		t.Fatal("expected a real limiter")
	}
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRateLimiterWaitBlocked(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{Limit: 1, Burst: 1})
	if err := rl.Wait(context.Background()); err != nil {
		t.Fatalf("first token should be granted: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := rl.Wait(ctx); err == nil {
		t.Fatal("expected error when waiting for the second token")
	}
}

func TestRateLimiterWaitCanceled(t *testing.T) {
	rl := NewRateLimiter(&RateLimiterConfig{Limit: 1, Burst: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := rl.Wait(ctx); err == nil {
		t.Fatal("expected error for canceled context")
	}
}
