package httpclient

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type recorderLogger struct {
	mu   sync.Mutex
	msgs []string
}

func (l *recorderLogger) DebugF(format string, args ...any) { l.log(fmt.Sprintf(format, args...)) }
func (l *recorderLogger) Debug(msg string)                  { l.log(msg) }
func (l *recorderLogger) Info(msg string)                   { l.log(msg) }
func (l *recorderLogger) Warn(msg string)                   { l.log(msg) }
func (l *recorderLogger) Error(msg string)                  { l.log(msg) }

func (l *recorderLogger) log(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, msg)
}

func (l *recorderLogger) contains(substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, msg := range l.msgs {
		if contains(msg, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

type countingTransport struct {
	mu    sync.Mutex
	calls int
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.Lock()
	t.calls++
	t.mu.Unlock()
	return http.DefaultTransport.RoundTrip(req)
}

func (t *countingTransport) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.calls
}

func testConfig() *Config {
	return &Config{
		Env:             "test",
		Address:         "http://localhost",
		MaxIdleConns:    100,
		RequestTimeout:  10 * time.Second,
		IdleConnTimeout: 30 * time.Second,
		CircuitBreakerConfig: CircuitBreakerConfig{
			Name: "test-breaker",
		},
		RateLimiterConfig: RateLimiterConfig{
			Limit: 1000,
			Burst: 10,
		},
	}
}

const fixedRquid = "test-rquid-0001"

func fixedRequestID() string { return fixedRquid }

func generateCertFiles(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatalf("failed to write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}
	return certFile, keyFile
}
