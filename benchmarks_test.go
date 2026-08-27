package httpclient

import (
	"context"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var benchmarkSink any

func benchmarkServer(tb testing.TB) *httptest.Server {
	tb.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"hello":"world"}`)
	}))
	tb.Cleanup(srv.Close)
	return srv
}

func benchmarkClient(b *testing.B, mut func(*Config)) *Client {
	b.Helper()
	cfg := testConfig()
	if mut != nil {
		mut(cfg)
	}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		b.Fatalf("failed to create client: %v", err)
	}
	return client
}

func BenchmarkClientGet(b *testing.B) {
	srv := benchmarkServer(b)
	client := benchmarkClient(b, nil)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.Get(ctx, srv.URL+"/bench", nil); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkClientGetWithRetry(b *testing.B) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls%10 == 0 {
			http.Error(w, "flaky", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	client := benchmarkClient(b, func(cfg *Config) {
		cfg.RetryerConfig = RetryerConfig{Attempts: 2, Delay: time.Microsecond}
	})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.Get(ctx, srv.URL+"/flaky", nil); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkClientPost(b *testing.B) {
	srv := benchmarkServer(b)
	client := benchmarkClient(b, nil)
	ctx := context.Background()
	body := strings.NewReader("benchmark payload")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = body.Seek(0, io.SeekStart)
		if _, err := client.Post(ctx, srv.URL+"/bench", body, nil); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkRetryerSuccess(b *testing.B) {
	r := newRetryer(RetryerConfig{Attempts: 3, Delay: time.Microsecond}, NoopLogger{}, rand.New(rand.NewSource(1)))
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := r.Do(ctx, func() (*http.Response, error) {
			return fakeResponse(http.StatusOK), nil
		}); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkRateLimiterWait(b *testing.B) {
	rl := NewRateLimiter(&RateLimiterConfig{Limit: 1_000_000, Burst: b.N})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := rl.Wait(ctx); err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}

func BenchmarkRetryerJitter(b *testing.B) {
	r := newRetryer(RetryerConfig{}, NoopLogger{}, rand.New(rand.NewSource(1)))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSink = r.jitter(100 * time.Millisecond)
	}
}

func BenchmarkClientCircuitBreakerOpen(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := benchmarkClient(b, func(cfg *Config) {
		cfg.CircuitBreakerConfig = CircuitBreakerConfig{Name: "bench", Counts: 1, MaxRequests: 1, Timeout: time.Hour}
	})
	ctx := context.Background()

	if _, err := client.Get(ctx, srv.URL, nil); err == nil {
		b.Fatal("expected first request to trip the breaker")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := client.Get(ctx, srv.URL, nil); err == nil {
			b.Fatal("expected open-state rejection")
		}
	}
}
