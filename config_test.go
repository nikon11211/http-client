package httpclient

import (
	"crypto/tls"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr error
	}{
		{name: "nil config", config: nil, wantErr: ErrConfigNil},
		{name: "empty config", config: &Config{}, wantErr: ErrInvalidConfig},
		{name: "missing max idle conns", config: &Config{Address: "http://localhost"}, wantErr: ErrInvalidConfig},
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
		{
			name: "valid",
			config: &Config{
				Address:         "http://localhost",
				MaxIdleConns:    10,
				RequestTimeout:  time.Second,
				IdleConnTimeout: time.Second,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error wrapping %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestConfigValidateAddressWhitespace(t *testing.T) {
	cfg := &Config{
		Address:         "   ",
		MaxIdleConns:    10,
		RequestTimeout:  time.Second,
		IdleConnTimeout: time.Second,
	}
	err := cfg.validate()
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for whitespace address, got: %v", err)
	}
}

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		version string
		want    uint16
	}{
		{version: "1.0", want: tls.VersionTLS10},
		{version: "1.1", want: tls.VersionTLS11},
		{version: "1.2", want: tls.VersionTLS12},
		{version: "1.3", want: tls.VersionTLS13},
		{version: "bogus", want: tls.VersionTLS12},
		{version: "", want: tls.VersionTLS12},
	}
	for _, tt := range tests {
		if got := parseTLSVersion(tt.version); got != tt.want {
			t.Errorf("parseTLSVersion(%q) = %d, want %d", tt.version, got, tt.want)
		}
	}
}

func TestErrorsAreDistinct(t *testing.T) {
	all := []error{
		ErrConfigNil,
		ErrInvalidConfig,
		ErrInvalidRequest,
		ErrInvalidAuthType,
		ErrRateLimitWait,
		ErrCircuitOpen,
		ErrRequestFailed,
		ErrNonSuccessStatus,
		ErrResponseTooLarge,
		ErrRetriesExhausted,
		ErrTLSCertLoad,
		ErrCAInvalid,
	}
	seen := map[string]bool{}
	for _, err := range all {
		if seen[err.Error()] {
			t.Errorf("duplicate error message: %q", err.Error())
		}
		seen[err.Error()] = true
		if !strings.Contains(err.Error(), "httpclient") {
			t.Errorf("error %q does not mention httpclient", err.Error())
		}
	}
}
