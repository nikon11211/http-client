package httpclient

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"
)

func fakeResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     http.Header{},
	}
}

func TestRetryerSingleAttemptByDefault(t *testing.T) {
	r := newRetryer(RetryerConfig{}, NoopLogger{}, rand.New(rand.NewSource(1)))
	calls := 0
	_, err := r.Do(context.Background(), func() (*http.Response, error) {
		calls++
		return nil, errors.New("network error")
	})
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected ErrRetriesExhausted, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryerNoRetryOnSuccess(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 3, Delay: time.Millisecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	calls := 0
	resp, err := r.Do(context.Background(), func() (*http.Response, error) {
		calls++
		return fakeResponse(http.StatusOK), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryerNoRetryOnClientError(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 3, Delay: time.Millisecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	calls := 0
	resp, err := r.Do(context.Background(), func() (*http.Response, error) {
		calls++
		return fakeResponse(http.StatusBadRequest), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryerRetriesNetworkErrors(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 3, Delay: time.Millisecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	calls := 0
	resp, err := r.Do(context.Background(), func() (*http.Response, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("connection refused")
		}
		return fakeResponse(http.StatusOK), nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRetryerRetriesServerErrors(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 3, Delay: time.Millisecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	calls := 0
	resp, err := r.Do(context.Background(), func() (*http.Response, error) {
		calls++
		if calls < 3 {
			return fakeResponse(http.StatusServiceUnavailable), nil
		}
		return fakeResponse(http.StatusOK), nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRetryerExhaustsNetworkErrors(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 3, Delay: time.Millisecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	calls := 0
	_, err := r.Do(context.Background(), func() (*http.Response, error) {
		calls++
		return nil, errors.New("connection refused")
	})
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected ErrRetriesExhausted, got: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryerExhaustsServerErrors(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 2, Delay: time.Millisecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	calls := 0
	_, err := r.Do(context.Background(), func() (*http.Response, error) {
		calls++
		return fakeResponse(http.StatusInternalServerError), nil
	})
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected ErrRetriesExhausted, got: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRetryerContextAlreadyCanceled(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 3, Delay: time.Millisecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, err := r.Do(ctx, func() (*http.Response, error) {
		calls++
		return nil, errors.New("boom")
	})
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected ErrRetriesExhausted, got: %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected 0 calls, got %d", calls)
	}
}

func TestRetryerContextCanceledDuringSleep(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 3, Delay: 100 * time.Millisecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	_, err := r.Do(ctx, func() (*http.Response, error) {
		calls++
		if calls == 1 {
			time.AfterFunc(5*time.Millisecond, cancel)
		}
		return nil, errors.New("connection refused")
	})
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected ErrRetriesExhausted, got: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryerJitterRange(t *testing.T) {
	r := newRetryer(RetryerConfig{}, NoopLogger{}, rand.New(rand.NewSource(1)))
	base := 100 * time.Millisecond
	for i := 0; i < 10; i++ {
		got := r.jitter(base)
		if got < base/2 || got > base {
			t.Fatalf("jitter %v out of range [%v, %v]", got, base/2, base)
		}
	}
}

func TestRetryerJitterZeroDelay(t *testing.T) {
	r := newRetryer(RetryerConfig{}, NoopLogger{}, rand.New(rand.NewSource(1)))
	if got := r.jitter(0); got != 0 {
		t.Fatalf("expected zero jitter for zero delay, got %v", got)
	}
}

func TestRetryerSleepZeroDelay(t *testing.T) {
	r := newRetryer(RetryerConfig{}, NoopLogger{}, rand.New(rand.NewSource(1)))
	if !r.sleep(context.Background(), 0) {
		t.Fatal("expected sleep with zero delay to succeed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r.sleep(ctx, 0) {
		t.Fatal("expected sleep with zero delay to fail on canceled context")
	}
}

func TestRetryerSleepTimer(t *testing.T) {
	r := newRetryer(RetryerConfig{}, NoopLogger{}, rand.New(rand.NewSource(1)))
	if !r.sleep(context.Background(), time.Millisecond) {
		t.Fatal("expected timer sleep to succeed")
	}
}

func TestRetryerLoggerDefaultsToNoop(t *testing.T) {
	r := newRetryer(RetryerConfig{Attempts: 2, Delay: time.Millisecond}, nil, rand.New(rand.NewSource(1)))
	if r.logger == nil {
		t.Fatal("expected non-nil logger")
	}
	resp, err := r.Do(context.Background(), func() (*http.Response, error) {
		return fakeResponse(http.StatusOK), nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
}
