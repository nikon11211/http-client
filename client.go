package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sony/gobreaker"
)

type Client struct {
	config    *Config
	logger    Logger
	http      *http.Client
	breaker   *CircuitBreaker
	limiter   *RateLimiter
	retryer   *Retryer
	requestID func() string
}

type Response struct {
	StatusCode    int
	Status        string
	Header        http.Header
	Body          []byte
	Size          int64
	ContentLength int64
	RequestID     string
	Duration      time.Duration
	Attempts      int
}

func (r *Response) ContentType() string {
	return r.Header.Get("Content-Type")
}

func New(config *Config, logger Logger, opts ...Option) (*Client, error) {
	const trace = "client.New"

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", trace, err)
	}
	if logger == nil {
		logger = NoopLogger{}
	}

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	tlsConfig, err := buildTLSConfig(config.TLSConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", trace, err)
	}

	httpClient := o.httpClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: config.RequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:       config.MaxIdleConns,
				IdleConnTimeout:    config.IdleConnTimeout,
				TLSClientConfig:    tlsConfig,
				DisableCompression: config.DisableCompression,
				DisableKeepAlives:  config.DisableKeepAlives,
			},
		}
	}

	requestID := uuid.NewString
	if o.requestID != nil {
		requestID = o.requestID
	}

	return &Client{
		config:    config,
		logger:    logger,
		http:      httpClient,
		breaker:   NewCircuitBreaker(&config.CircuitBreakerConfig, onStateChange(logger)),
		limiter:   NewRateLimiter(&config.RateLimiterConfig),
		retryer:   NewRetryer(config.RetryerConfig, logger),
		requestID: requestID,
	}, nil
}

func (c *Client) Do(ctx context.Context, method, url string, body io.Reader, headers http.Header) (*Response, error) {
	const trace = "client.do"
	if ctx == nil {
		ctx = context.Background()
	}
	rquid := c.requestID()
	start := time.Now()

	if err := c.limiter.Wait(ctx); err != nil {
		c.logger.Error(fmt.Sprintf("(%s) [%s] rate limiter wait failed: %v", trace, rquid, err))
		return nil, fmt.Errorf("%w: %w", ErrRateLimitWait, err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		c.logger.Error(fmt.Sprintf("(%s) [%s] failed to create request: %v", trace, rquid, err))
		return nil, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	req.Header.Set("X-Request-ID", rquid)
	c.applyHeaders(req, headers)
	if err := c.applyAuth(req); err != nil {
		c.logger.Error(fmt.Sprintf("(%s) [%s] failed to apply authentication: %v", trace, rquid, err))
		return nil, err
	}

	attempts := 0
	breakerResult, err := c.breaker.Execute(func() (any, error) {
		return c.retryer.Do(ctx, func() (*http.Response, error) {
			attempts++
			return c.http.Do(req)
		})
	})
	if err != nil {
		c.logger.Error(fmt.Sprintf("(%s) [%s] request failed: %v", trace, rquid, err))
		if errors.Is(err, gobreaker.ErrOpenState) {
			return nil, fmt.Errorf("%w: %w", ErrCircuitOpen, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrRequestFailed, err)
	}
	rawResp := breakerResult.(*http.Response)
	defer func() { _ = rawResp.Body.Close() }()

	bodyBytes, size, err := c.readBody(rawResp.Body)
	if err != nil {
		c.logger.Error(fmt.Sprintf("(%s) [%s] failed to read response body: %v", trace, rquid, err))
		return nil, err
	}

	result := &Response{
		StatusCode:    rawResp.StatusCode,
		Status:        rawResp.Status,
		Header:        rawResp.Header.Clone(),
		Body:          bodyBytes,
		Size:          size,
		ContentLength: rawResp.ContentLength,
		RequestID:     rquid,
		Duration:      time.Since(start),
		Attempts:      attempts,
	}

	if err := c.handleResponse(rquid, rawResp); err != nil {
		return result, err
	}

	c.logger.Info(fmt.Sprintf("(%s) [%s] %s %s completed in %s, response size: %.3f MB",
		trace, rquid, method, url, result.Duration, c.toMGb(int(size))))
	return result, nil
}

func (c *Client) Get(ctx context.Context, url string, headers http.Header) (*Response, error) {
	return c.Do(ctx, http.MethodGet, url, nil, headers)
}

func (c *Client) Post(ctx context.Context, url string, body io.Reader, headers http.Header) (*Response, error) {
	return c.Do(ctx, http.MethodPost, url, body, headers)
}

func (c *Client) handleResponse(rquid string, response *http.Response) error {
	const trace = "client.handleResponse"
	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		c.logger.Info(fmt.Sprintf("(%s) [%s] received response status code: %d", trace, rquid, response.StatusCode))
		return nil
	default:
		c.logger.Error(fmt.Sprintf("(%s) [%s] received response status code: %d", trace, rquid, response.StatusCode))
		return fmt.Errorf("%w: status code %d", ErrNonSuccessStatus, response.StatusCode)
	}
}

func (c *Client) readBody(body io.Reader) ([]byte, int64, error) {
	if c.config.MaxResponseBytes <= 0 {
		data, err := io.ReadAll(body)
		return data, int64(len(data)), err
	}
	data, err := io.ReadAll(io.LimitReader(body, c.config.MaxResponseBytes+1))
	if int64(len(data)) > c.config.MaxResponseBytes {
		return nil, 0, fmt.Errorf("%w: limit is %d bytes", ErrResponseTooLarge, c.config.MaxResponseBytes)
	}
	return data, int64(len(data)), err
}

func (c *Client) toMGb(bytes int) float64 {
	const bytesInMB = 1024 * 1024
	return float64(bytes) / bytesInMB
}

func (c *Client) applyHeaders(req *http.Request, headers http.Header) {
	if headers == nil {
		return
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}

func (c *Client) applyAuth(req *http.Request) error {
	auth := c.config.AuthConfig
	switch strings.ToLower(auth.Type) {
	case "", "basic":
		if auth.Username != "" {
			req.SetBasicAuth(auth.Username, auth.Password)
		}
	case "bearer":
		if auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		}
	default:
		return fmt.Errorf("%w: %q", ErrInvalidAuthType, auth.Type)
	}
	return nil
}

func buildTLSConfig(cfg TLSConfig, logger Logger) (*tls.Config, error) {
	const trace = "client.buildTLSConfig"
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	if !cfg.Enabled {
		logger.Info(fmt.Sprintf("(%s) TLS disabled", trace))
		return tlsConfig, nil
	}
	if cfg.MinVersion != "" {
		tlsConfig.MinVersion = parseTLSVersion(cfg.MinVersion)
	}
	if cfg.MaxVersion != "" {
		tlsConfig.MaxVersion = parseTLSVersion(cfg.MaxVersion)
	}
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			logger.Error(fmt.Sprintf("(%s) failed to load TLS certificate: %v", trace, err))
			return nil, fmt.Errorf("%w: %w", ErrTLSCertLoad, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		logger.Info(fmt.Sprintf("(%s) loaded client certificate from %s", trace, cfg.CertFile))
	}
	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			logger.Error(fmt.Sprintf("(%s) failed to read CA certificate: %v", trace, err))
			return nil, fmt.Errorf("%w: %w", ErrTLSCertLoad, err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caPEM) {
			logger.Error(fmt.Sprintf("(%s) failed to parse CA certificate from %s", trace, cfg.CAFile))
			return nil, fmt.Errorf("%w: %s", ErrCAInvalid, cfg.CAFile)
		}
		tlsConfig.RootCAs = caPool
		logger.Info(fmt.Sprintf("(%s) loaded CA certificate chain from %s", trace, cfg.CAFile))
	}
	logger.Info(fmt.Sprintf("(%s) TLS enabled (min: %s, max: %s)", trace, cfg.MinVersion, cfg.MaxVersion))
	return tlsConfig, nil
}

func parseTLSVersion(version string) uint16 {
	switch version {
	case "1.0":
		return tls.VersionTLS10
	case "1.1":
		return tls.VersionTLS11
	case "1.2":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	default:
		return tls.VersionTLS12
	}
}

func onStateChange(logger Logger) func(name string, from, to gobreaker.State) {
	return func(name string, from, to gobreaker.State) {
		logger.Warn(fmt.Sprintf("(circuit_breaker) '%s' state changed from %s to %s", name, from, to))
	}
}
