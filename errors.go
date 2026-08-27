package httpclient

import "errors"

var (
	ErrConfigNil        = errors.New("httpclient: config is nil")
	ErrInvalidConfig    = errors.New("httpclient: invalid config")
	ErrInvalidRequest   = errors.New("httpclient: invalid request")
	ErrInvalidAuthType  = errors.New("httpclient: unsupported auth type")
	ErrRateLimitWait    = errors.New("httpclient: rate limit wait failed")
	ErrCircuitOpen      = errors.New("httpclient: circuit breaker is open")
	ErrRequestFailed    = errors.New("httpclient: request failed")
	ErrNonSuccessStatus = errors.New("httpclient: non-success status code")
	ErrResponseTooLarge = errors.New("httpclient: response body exceeds max response bytes")
	ErrRetriesExhausted = errors.New("httpclient: retries exhausted")
	ErrTLSCertLoad      = errors.New("httpclient: failed to load TLS certificate")
	ErrCAInvalid        = errors.New("httpclient: invalid CA certificate")
)
