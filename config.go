package httpclient

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	Env                  string               `mapstructure:"env"`
	Address              string               `mapstructure:"address"`
	MaxIdleConns         int                  `mapstructure:"max_idle_conns"`
	RequestTimeout       time.Duration        `mapstructure:"request_timeout"`
	IdleConnTimeout      time.Duration        `mapstructure:"idle_conn_timeout"`
	MaxResponseBytes     int64                `mapstructure:"max_response_bytes"`
	DisableCompression   bool                 `mapstructure:"disable_compression"`
	DisableKeepAlives    bool                 `mapstructure:"disable_keep_alives"`
	CircuitBreakerConfig CircuitBreakerConfig `mapstructure:"circuit_breaker"`
	RateLimiterConfig    RateLimiterConfig    `mapstructure:"rate_limiter"`
	RetryerConfig        RetryerConfig        `mapstructure:"retryer"`
	AuthConfig           AuthConfig           `mapstructure:"auth"`
	TLSConfig            TLSConfig            `mapstructure:"tls"`
}

type CircuitBreakerConfig struct {
	Name        string        `mapstructure:"name"`
	MaxRequests uint32        `mapstructure:"max_requests"`
	Counts      uint32        `mapstructure:"counts"`
	Interval    time.Duration `mapstructure:"interval"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type RateLimiterConfig struct {
	Limit int `mapstructure:"limit"`
	Burst int `mapstructure:"burst"`
}

type RetryerConfig struct {
	Attempts int           `mapstructure:"attempts"`
	Delay    time.Duration `mapstructure:"delay"`
}

type TLSConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	MinVersion         string `mapstructure:"min_version"`
	MaxVersion         string `mapstructure:"max_version"`
	CertFile           string `mapstructure:"cert_file"`
	KeyFile            string `mapstructure:"key_file"`
	CAFile             string `mapstructure:"ca_file"`
	InsecureSkipVerify bool   `mapstructure:"insecure_skip_verify"`
}

type AuthConfig struct {
	Type     string `mapstructure:"type"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	Token    string `mapstructure:"token"`
}

func (c *Config) validate() error {
	if c == nil {
		return ErrConfigNil
	}
	if strings.TrimSpace(c.Address) == "" {
		return fmt.Errorf("%w: address is required", ErrInvalidConfig)
	}
	if c.MaxIdleConns <= 0 {
		return fmt.Errorf("%w: max_idle_conns must be positive", ErrInvalidConfig)
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("%w: request_timeout must be positive", ErrInvalidConfig)
	}
	if c.IdleConnTimeout <= 0 {
		return fmt.Errorf("%w: idle_conn_timeout must be positive", ErrInvalidConfig)
	}
	return nil
}
