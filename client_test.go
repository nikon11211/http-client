package httpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr error
	}{
		{name: "nil config", config: nil, wantErr: ErrConfigNil},
		{name: "empty config", config: &Config{}, wantErr: ErrInvalidConfig},
		{
			name:    "missing max idle conns",
			config:  &Config{Address: "http://localhost"},
			wantErr: ErrInvalidConfig,
		},
		{
			name:    "missing request timeout",
			config:  &Config{Address: "http://localhost", MaxIdleConns: 10},
			wantErr: ErrInvalidConfig,
		},
		{
			name: "missing idle conn timeout",
			config: &Config{
				Address:        "http://localhost",
				MaxIdleConns:   10,
				RequestTimeout: time.Second,
			},
			wantErr: ErrInvalidConfig,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := New(tt.config, NoopLogger{})
			if client != nil {
				t.Fatalf("expected nil client, got: %+v", client)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error wrapping %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewWithNilLogger(t *testing.T) {
	client, err := New(testConfig(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if _, ok := client.logger.(NoopLogger); !ok {
		t.Fatalf("expected NoopLogger, got: %T", client.logger)
	}
}

func TestClientGetSuccess(t *testing.T) {
	var gotRequestID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = r.Header.Get("X-Request-ID")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"hello":"world"}`)
	}))
	defer srv.Close()

	logger := &recorderLogger{}
	cfg := testConfig()
	client, err := New(cfg, logger, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := client.Get(context.Background(), srv.URL+"/hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if resp.Status != "200 OK" {
		t.Fatalf("expected '200 OK', got %q", resp.Status)
	}
	if gotRequestID != fixedRquid {
		t.Fatalf("expected X-Request-ID %q, got %q", fixedRquid, gotRequestID)
	}
	if resp.RequestID != fixedRquid {
		t.Fatalf("expected request ID %q, got %q", fixedRquid, resp.RequestID)
	}
	if string(resp.Body) != `{"hello":"world"}` {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
	if resp.Size != int64(len(resp.Body)) {
		t.Fatalf("expected size %d, got %d", len(resp.Body), resp.Size)
	}
	if resp.ContentLength != int64(len(resp.Body)) {
		t.Fatalf("expected content length %d, got %d", len(resp.Body), resp.ContentLength)
	}
	if resp.ContentType() != "application/json" {
		t.Fatalf("expected content type application/json, got %q", resp.ContentType())
	}
	if resp.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", resp.Attempts)
	}
	if resp.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", resp.Duration)
	}
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type header on response, got %v", resp.Header)
	}
	if !logger.contains(fixedRquid) {
		t.Fatal("expected logger to receive the request id")
	}
	if !logger.contains("received response status code: 200") {
		t.Fatal("expected success status log")
	}
	if !logger.contains("completed") {
		t.Fatal("expected completion log")
	}
}

func TestClientDefaultRequestID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			t.Error("expected non-empty X-Request-ID header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := New(testConfig(), NoopLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, err := client.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RequestID == "" {
		t.Fatal("expected non-empty request ID")
	}
}

func TestClientPostEcho(t *testing.T) {
	var gotMethod, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client, err := New(testConfig(), NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	headers := http.Header{"Content-Type": []string{"text/plain"}}
	resp, err := client.Post(context.Background(), srv.URL+"/echo", bytes.NewBufferString("payload"), headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotBody != "payload" {
		t.Fatalf("expected body 'payload', got %q", gotBody)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
}

func TestClientNotFoundReturnsStructuredResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client, err := New(testConfig(), NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp, err := client.Get(context.Background(), srv.URL+"/missing", nil)
	if !errors.Is(err, ErrNonSuccessStatus) {
		t.Fatalf("expected ErrNonSuccessStatus, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected a response even on error")
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestClientRetryOn5xxThenSuccess(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.RetryerConfig = RetryerConfig{Attempts: 3, Delay: time.Millisecond}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := client.Get(context.Background(), srv.URL+"/flaky", nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", calls.Load())
	}
	if resp.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", resp.Attempts)
	}
	if string(resp.Body) != "ok" {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
}

func TestClientRetryExhaustedOn5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.RetryerConfig = RetryerConfig{Attempts: 2, Delay: time.Millisecond}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := client.Get(context.Background(), srv.URL, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if resp != nil {
		t.Fatalf("expected nil response, got: %+v", resp)
	}
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("expected ErrRequestFailed, got: %v", err)
	}
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected wrapped ErrRetriesExhausted, got: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", calls.Load())
	}
}

func TestClientNetworkErrorRetries(t *testing.T) {
	cfg := testConfig()
	cfg.RetryerConfig = RetryerConfig{Attempts: 3, Delay: time.Millisecond}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.Get(context.Background(), "http://127.0.0.1:1/unreachable", nil)
	if err == nil {
		t.Fatal("expected error for unreachable address")
	}
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("expected ErrRequestFailed, got: %v", err)
	}
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected wrapped ErrRetriesExhausted, got: %v", err)
	}
}

func TestClientMaxResponseBytesExceeded(t *testing.T) {
	body := strings.Repeat("x", 2048)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.MaxResponseBytes = 1024
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.Get(context.Background(), srv.URL, nil)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("expected ErrResponseTooLarge, got: %v", err)
	}
}

func TestClientMaxResponseBytesWithinLimit(t *testing.T) {
	body := strings.Repeat("y", 512)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.MaxResponseBytes = 1024
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := client.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Size != int64(len(body)) {
		t.Fatalf("expected size %d, got %d", len(body), resp.Size)
	}
}

func TestClientTLSWithInsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "secure")
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{Enabled: true, InsecureSkipVerify: true}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := client.Get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error over TLS: %v", err)
	}
	if string(resp.Body) != "secure" {
		t.Fatalf("unexpected body: %q", string(resp.Body))
	}
}

func TestClientAuthBasic(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.AuthConfig = AuthConfig{Type: "basic", Username: "user", Password: "pass"}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
	if gotAuth != want {
		t.Fatalf("expected %q, got %q", want, gotAuth)
	}
}

func TestClientAuthBearer(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.AuthConfig = AuthConfig{Type: "bearer", Token: "tok123"}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer tok123" {
		t.Fatalf("expected 'Bearer tok123', got %q", gotAuth)
	}
}

func TestClientAuthInvalidType(t *testing.T) {
	cfg := testConfig()
	cfg.AuthConfig = AuthConfig{Type: "api_key"}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err = client.Get(context.Background(), srv.URL, nil)
	if !errors.Is(err, ErrInvalidAuthType) {
		t.Fatalf("expected ErrInvalidAuthType, got: %v", err)
	}
}

func TestClientRateLimitWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.RateLimiterConfig = RateLimiterConfig{Limit: 1, Burst: 1}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatalf("first request should pass: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = client.Get(ctx, srv.URL, nil)
	if !errors.Is(err, ErrRateLimitWait) {
		t.Fatalf("expected ErrRateLimitWait, got: %v", err)
	}
}

func TestClientCircuitBreakerOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.CircuitBreakerConfig = CircuitBreakerConfig{
		Name:        "test-breaker",
		MaxRequests: 1,
		Counts:      1,
		Timeout:     time.Hour,
	}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.Get(context.Background(), srv.URL, nil); err == nil {
		t.Fatal("expected first request to fail")
	}
	_, err = client.Get(context.Background(), srv.URL, nil)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestClientCancelledContext(t *testing.T) {
	client, err := New(testConfig(), NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Get(ctx, "http://localhost/unused", nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestClientInvalidURL(t *testing.T) {
	client, err := New(testConfig(), NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = client.Get(context.Background(), "://bad-url", nil)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got: %v", err)
	}
}

func TestClientNilContextDefaultsToBackground(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := New(testConfig(), NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := client.Do(nil, http.MethodGet, srv.URL, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientCustomHTTPClientOption(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &countingTransport{}
	httpClient := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	client, err := New(testConfig(), NoopLogger{}, WithHTTPClient(httpClient), WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := client.Get(context.Background(), srv.URL, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if transport.count() != 1 {
		t.Fatalf("expected 1 round trip, got %d", transport.count())
	}
}

func TestClientCustomHeaders(t *testing.T) {
	var gotCustom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCustom = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := New(testConfig(), NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	headers := http.Header{"X-Custom": []string{"value-1", "value-2"}}
	if _, err := client.Get(context.Background(), srv.URL, headers); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCustom != "value-1" {
		t.Fatalf("expected 'value-1', got %q", gotCustom)
	}
}
