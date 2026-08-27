<p align="center"><h1 align="center">🚀 HTTP-Client — Enterprise-Grade HTTP Client Library for Go</h1></p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/nikon11211/http-client">
    <img src="https://pkg.go.dev/badge/github.com/nikon11211/http-client.svg" alt="Go Reference"/>
  </a>
  <a href="https://goreportcard.com/report/github.com/nikon11211/http-client">
    <img src="https://goreportcard.com/badge/github.com/nikon11211/http-client" alt="Go Report Card"/>
  </a>
  <a href="https://github.com/nikon11211/http-client/actions/workflows/test.yaml">
    <img src="https://github.com/nikon11211/http-client/actions/workflows/test.yaml/badge.svg" alt="Tests"/>
  </a>
  <a href="https://codecov.io/gh/nikon11211/http-client">
    <img src="https://codecov.io/gh/nikon11211/http-client/branch/main/graph/badge.svg" alt="Coverage"/>
  </a>
  <a href="https://sonarcloud.io/summary/overall?id=nikon11211_http-client">
    <img src="https://sonarcloud.io/api/project_badges/measure?project=nikon11211_http-client&metric=coverage" alt="SonarCloud Coverage"/>
  </a>
  <a href="https://opensource.org/licenses/MIT">
    <img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"/>
  </a>
  <a href="https://golang.org/">
    <img src="https://img.shields.io/badge/Go-%3E%3D%201.26-blue" alt="Go Version"/>
  </a>
</p>

<p align="center">
  <b>A production-ready, resilient HTTP client for Go microservices</b><br/>
  <i>Circuit breaker • Rate limiting • Retries with jitter • TLS • Auth • Structured responses</i>
</p>

---

## ✨ Why HTTP-Client?

Calling HTTP services from Go microservices seems easy — until the service
goes down, the network flakes, or a slow endpoint trips your whole process.
This library wraps `net/http` with everything a production client needs:

- a **circuit breaker** that stops hammering a failing service,
- a **rate limiter** that protects upstream APIs,
- **automatic retries** with jittered backoff for 5xx and network errors,
- **TLS** with client certificates and custom CA trust stores,
- **basic/bearer authentication** and per-request correlation IDs.

It is dependency-light (only `sony/gobreaker`, `golang.org/x/time`,
`google/uuid`), has **zero corporate dependencies**, and ships with **100%
statement test coverage**.

---

## 🎯 Features

<table>
<tr>
<td width="50%">

### 🚀 Core Features
- Structured `Response` (status, headers, body, request ID, duration, attempts)
- Automatic retries with jittered backoff (50–100% of base delay)
- Retry on HTTP 5xx responses and network errors
- Rate limiting via `golang.org/x/time/rate` with configurable limit/burst
- Circuit breaking via `sony/gobreaker` with state-change logging
- Response body size limit (`MaxResponseBytes`) against runaway payloads
- UUID request IDs (default) sent as `X-Request-ID` and logged
- `Get` / `Post` / `Do` API with `context` support

</td>
<td width="50%">

### 📊 Observability
- Minimal `Logger` interface with a built-in `NoopLogger`
- Works with slog, zap, logrus or any structured logger via a thin wrapper
- Request IDs attached to every log message
- Response size logged in megabytes
- Circuit breaker state transitions logged on every change

</td>
</tr>
</table>

---

## 📦 Installation

```bash
go get github.com/nikon11211/http-client
```

Then import it as `httpclient`:

```go
import httpclient "github.com/nikon11211/http-client"
```

## 🚀 Quick Start

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/nikon11211/http-client"
)

func main() {
	cfg := &httpclient.Config{
		Address:          "https://httpbin.org",
		MaxIdleConns:     100,
		RequestTimeout:   30 * time.Second,
		IdleConnTimeout:  90 * time.Second,
		MaxResponseBytes: 1 << 20,
		CircuitBreakerConfig: httpclient.CircuitBreakerConfig{
			Name:    "httpbin",
			Counts:  5,
			Timeout: 30 * time.Second,
		},
		RateLimiterConfig: httpclient.RateLimiterConfig{
			Limit: 20,
			Burst: 5,
		},
		RetryerConfig: httpclient.RetryerConfig{
			Attempts: 3,
			Delay:    500 * time.Millisecond,
		},
	}

	client, err := httpclient.New(cfg, nil) // nil -> NoopLogger
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, cfg.Address+"/get", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("status=%d request_id=%s attempts=%d size=%d\n",
		resp.StatusCode, resp.RequestID, resp.Attempts, resp.Size)
}
```

### Structured responses

A non-2xx status still returns a populated `*Response` alongside the error:

```go
resp, err := client.Get(ctx, url, nil)
if errors.Is(err, httpclient.ErrNonSuccessStatus) {
	fmt.Println("got status", resp.StatusCode, "with body", string(resp.Body))
}
```

### Custom logger (e.g. slog)

```go
type slogAdapter struct{ log *slog.Logger } // implements httpclient.Logger

func (a *slogAdapter) DebugF(format string, args ...any) { a.log.Debug(fmt.Sprintf(format, args...)) }
func (a *slogAdapter) Debug(msg string)                  { a.log.Debug(msg) }
func (a *slogAdapter) Info(msg string)                   { a.log.Info(msg) }
func (a *slogAdapter) Warn(msg string)                   { a.log.Warn(msg) }
func (a *slogAdapter) Error(msg string)                  { a.log.Error(msg) }

client, err := httpclient.New(cfg, &slogAdapter{log: slog.New(slog.NewJSONHandler(os.Stdout, nil))})
```

### Retry on flaky endpoints

```go
cfg.RetryerConfig = httpclient.RetryerConfig{
	Attempts: 5,                    // total attempts, first included
	Delay:    200 * time.Millisecond, // jittered by 50-100%
}
```

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                        Your Application                      │
├──────────────────────────────────────────────────────────────┤
│                        httpclient.Client                     │
│                                                              │
│  ┌──────────────┐   ┌──────────────┐   ┌─────────────────┐   │
│  │ RateLimiter  │ → │CircuitBreaker│ → │ Retryer         │   │
│  │ x/time/rate  │   │sony/gobreaker│   │ jitter + ctx    │   │
│  └──────────────┘   └──────────────┘   └─────────────────┘   │
│                             │                                │
│                      net/http.Client                         │
│                     ┌──────────────┐                         │
│                     │   Transport  │                         │
│                     │ TLS • Auth   │                         │
│                     │  timeouts    │                         │
│                     └──────────────┘                         │
└──────────────────────────────────────────────────────────────┘
```

**Request lifecycle:**

1. A request ID is generated and attached as `X-Request-ID`.
2. The rate limiter waits for a token (aborting on context cancellation).
3. The circuit breaker admits the request unless it is open.
4. The retryer executes the HTTP call, retrying 5xx and network errors with
   jittered backoff until success, attempts are exhausted, or the context is
   canceled.
5. The response body is read with a `MaxResponseBytes` cap.
6. A structured `Response` is returned; non-2xx statuses also yield an error
   wrapping `ErrNonSuccessStatus`.

---

## ⚙️ Configuration

```go
type Config struct {
	Env                string               // deployment environment name
	Address            string               // base URL of the target service (required)
	MaxIdleConns       int                  // max idle connections in pool (required)
	RequestTimeout     time.Duration        // per-request timeout (required)
	IdleConnTimeout    time.Duration        // idle connection lifetime (required)
	MaxResponseBytes   int64                // response body cap; 0 = unlimited
	DisableCompression bool                 // disable transport compression
	DisableKeepAlives  bool                 // disable keep-alive connections
	CircuitBreakerConfig CircuitBreakerConfig // circuit breaker settings
	RateLimiterConfig  RateLimiterConfig    // rate limiter settings
	RetryerConfig      RetryerConfig        // retry settings
	AuthConfig         AuthConfig           // authentication settings
	TLSConfig          TLSConfig            // TLS settings
}
```

### CircuitBreakerConfig

| Field | Type | Meaning |
| ----- | ---- | ------- |
| `Name` | `string` | breaker name used in state-change logs (default `"httpclient"`) |
| `MaxRequests` | `uint32` | half-open probe requests |
| `Counts` | `uint32` | consecutive failures to trip; 0 = gobreaker default |
| `Interval` | `time.Duration` | failure-count reset period |
| `Timeout` | `time.Duration` | open-state duration before half-open |

### RateLimiterConfig

| Field | Type | Meaning |
| ----- | ---- | ------- |
| `Limit` | `int` | requests per second; 0 or negative = disabled |
| `Burst` | `int` | token bucket capacity; 0 = 1 |

### RetryerConfig

| Field | Type | Meaning |
| ----- | ---- | ------- |
| `Attempts` | `int` | total attempts (first included); 0 = a single attempt |
| `Delay` | `time.Duration` | base backoff delay, jittered to 50–100% |

### TLSConfig

| Field | Type | Meaning |
| ----- | ---- | ------- |
| `Enabled` | `bool` | enable TLS processing |
| `MinVersion` | `string` | `"1.0"` … `"1.3"` (default `"1.2"`) |
| `MaxVersion` | `string` | `"1.0"` … `"1.3"` (default: min version) |
| `CertFile` | `string` | client certificate path (mTLS) |
| `KeyFile` | `string` | client private key path (mTLS) |
| `CAFile` | `string` | custom CA bundle path (replaces system roots) |
| `InsecureSkipVerify` | `bool` | DANGER: skip verification (dev only) |

### AuthConfig

| Field | Type | Meaning |
| ----- | ---- | ------- |
| `Type` | `string` | `""`/`"basic"` or `"bearer"` (anything else → `ErrInvalidAuthType`) |
| `Username` | `string` | basic auth username |
| `Password` | `string` | basic auth password |
| `Token` | `string` | bearer token |

When `Type` is `"bearer"` the token is reused for every request; enabling
TLS is recommended whenever bearer tokens are in use.

---

## 🧩 Options

| Option | Description |
| ------ | ----------- |
| `WithHTTPClient(c *http.Client)` | Replace the default client entirely |
| `WithRequestIDGenerator(f func() string)` | Custom request ID source (default: `uuid.NewString`, UUID v4) |

Both have exact Go signatures, so they can be passed straight to `New`:

```go
client, err := httpclient.New(cfg, nil,
	httpclient.WithHTTPClient(custom),
	httpclient.WithRequestIDGenerator(gen),
)
```

---

## ❌ Errors

All failures wrap sentinel errors, so `errors.Is` works everywhere:

| Sentinel error | Raised when |
| ---------------- | ----------- |
| `ErrConfigNil` | `New(nil, ...)` is called |
| `ErrInvalidConfig` | Config validation fails (missing address / idles / timeouts) |
| `ErrInvalidRequest` | Request could not be constructed (e.g. bad URL) |
| `ErrInvalidAuthType` | Unknown auth type configured |
| `ErrRateLimitWait` | Rate limiter token wait failed (e.g. ctx canceled) |
| `ErrCircuitOpen` | Circuit breaker is open — request rejected |
| `ErrRequestFailed` | Request failed after all retries |
| `ErrNonSuccessStatus` | Response status is not in the 2xx range (`StatusCode` kept) |
| `ErrResponseTooLarge` | Body exceeded `MaxResponseBytes` |
| `ErrRetriesExhausted` | All retry attempts failed (also wraps the final error) |
| `ErrTLSCertLoad` | TLS cert/key could not be loaded |
| `ErrCAInvalid` | CA bundle could not be parsed |

---

## 🧪 Testing & Benchmarks

The library has **100.0% statement coverage** (race-enabled, atomic cover
mode) with no external services — every test runs against `httptest` servers.

```bash
# Run all tests
go test ./...

# Run with race detection and coverage (excluding examples)
go test -race -coverprofile=coverage.txt -covermode=atomic $(go list ./... | grep -v /examples)
go tool cover -func=coverage.txt | tail -3

# Run benchmarks
go test -bench=. -benchmem $(go list ./... | grep -v /examples)
```

Coverage includes: success / 4xx / 5xx-with-retry flows, body size limits,
TLS via `httptest.NewTLSServer`, basic/bearer auth, rate limiter waits and
circuit breaker open state.

Current benchmarks:

- `BenchmarkClientGet` — full GET round trip through the stack
- `BenchmarkClientGetWithRetry` — GET retried through a 5xx before success
- `BenchmarkClientPost` — full POST round trip
- `BenchmarkRetryerSuccess` — retryer fast-path with an immediate 2xx
- `BenchmarkRateLimitWait` — token acquisition from the rate limiter
- `BenchmarkRetryerJitter` — jittered backoff computation
- `BenchmarkClientCircuitBreakerOpen` — rejected request while breaker is open

`Benchmark*` functions use `b.ReportAllocs()` and keep results in a package
`benchmarkSink`, so benchmark output stays visible despite compiler
optimizations.

## 🤝 Contributing

We welcome contributions! Here's how you can help:

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

Keep tests deterministic (short retry delays, injectable random sources) and
preserve the 100% coverage gate.

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- [sony/gobreaker](https://github.com/sony/gobreaker) - circuit breaker implementation
- [golang.org/x/time](https://pkg.go.dev/golang.org/x/time/rate) - rate limiting
- [google/uuid](https://github.com/google/uuid) - request ID generation

---

<p align="center">
  <b>Made with ❤️ for the Go community</b><br/>
  <sub>Built for resilience, designed for production</sub>
</p>