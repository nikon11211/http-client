package httpclient

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewTLSDisabled(t *testing.T) {
	client, err := New(testConfig(), NoopLogger{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.config.TLSConfig.Enabled {
		t.Fatal("expected TLS to be disabled")
	}
}

func TestNewTLSEnabledWithoutFiles(t *testing.T) {
	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{Enabled: true}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewTLSMinMaxVersions(t *testing.T) {
	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{Enabled: true, MinVersion: "1.2", MaxVersion: "1.3"}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewTLSMissingCertFiles(t *testing.T) {
	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{
		Enabled:  true,
		CertFile: filepath.Join(t.TempDir(), "missing.pem"),
		KeyFile:  filepath.Join(t.TempDir(), "missing.key"),
	}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if client != nil {
		t.Fatalf("expected nil client, got: %+v", client)
	}
	if !errors.Is(err, ErrTLSCertLoad) {
		t.Fatalf("expected ErrTLSCertLoad, got: %v", err)
	}
}

func TestNewTLSWithCertFiles(t *testing.T) {
	certFile, keyFile := generateCertFiles(t, t.TempDir())
	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{
		Enabled:    true,
		MinVersion: "1.2",
		CertFile:   certFile,
		KeyFile:    keyFile,
	}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewTLSMissingCAFile(t *testing.T) {
	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{
		Enabled: true,
		CAFile:  filepath.Join(t.TempDir(), "missing-ca.pem"),
	}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if client != nil {
		t.Fatalf("expected nil client, got: %+v", client)
	}
	if !errors.Is(err, ErrTLSCertLoad) {
		t.Fatalf("expected ErrTLSCertLoad, got: %v", err)
	}
}

func TestNewTLSInvalidCAPEM(t *testing.T) {
	dir := t.TempDir()
	caFile := filepath.Join(dir, "bad-ca.pem")
	if err := os.WriteFile(caFile, []byte("this is not a pem"), 0o600); err != nil {
		t.Fatalf("failed to write CA file: %v", err)
	}
	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{
		Enabled: true,
		CAFile:  caFile,
	}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if client != nil {
		t.Fatalf("expected nil client, got: %+v", client)
	}
	if !errors.Is(err, ErrCAInvalid) {
		t.Fatalf("expected ErrCAInvalid, got: %v", err)
	}
}

func TestNewTLSWithCAFile(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := generateCertFiles(t, dir)
	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{
		Enabled:  true,
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   certFile,
	}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewTLSWithInsecureSkipVerifyOnly(t *testing.T) {
	cfg := testConfig()
	cfg.TLSConfig = TLSConfig{InsecureSkipVerify: true}
	client, err := New(cfg, NoopLogger{}, WithRequestIDGenerator(fixedRequestID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}
