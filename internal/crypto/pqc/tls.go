package pqc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// TLSMode configures the TLS security level.
type TLSMode int

const (
	// TLSModeServer configures server-side TLS with PQC.
	TLSModeServer TLSMode = iota
	// TLSModeClient configures client-side TLS with PQC.
	TLSModeClient
	// TLSModeMutual configures mutual TLS (mTLS) with PQC.
	TLSModeMutual
)

// NewTLSConfig creates a TLS 1.3 config with PQC-capable cipher suites.
// The hybrid X25519+ML-KEM-768 key exchange is enabled via Go's built-in
// crypto/tls support (Go 1.24+).
func NewTLSConfig(mode TLSMode) *tls.Config {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{
			tls.X25519MLKEM768, // Hybrid PQC: X25519 + ML-KEM-768
			tls.X25519,        // Fallback
			tls.CurveP256,     // Fallback
		},
	}

	switch mode {
	case TLSModeClient:
		// Client doesn't need certificates for basic TLS
		cfg.InsecureSkipVerify = false
	case TLSModeMutual:
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return cfg
}

// NewMutualTLSConfig creates a mutual TLS config with PQC support.
func NewMutualTLSConfig(caPool *x509.CertPool, cert tls.Certificate) *tls.Config {
	cfg := NewTLSConfig(TLSModeMutual)
	cfg.Certificates = []tls.Certificate{cert}
	cfg.ClientCAs = caPool
	cfg.RootCAs = caPool
	return cfg
}

// NewServerTLSConfig creates a server TLS config with the given certificate.
func NewServerTLSConfig(cert tls.Certificate) *tls.Config {
	cfg := NewTLSConfig(TLSModeServer)
	cfg.Certificates = []tls.Certificate{cert}
	return cfg
}

// NewClientTLSConfig creates a client TLS config with the given CA pool.
func NewClientTLSConfig(caPool *x509.CertPool) *tls.Config {
	cfg := NewTLSConfig(TLSModeClient)
	cfg.RootCAs = caPool
	return cfg
}

// LoadCertificate loads a TLS certificate from PEM-encoded files.
func LoadCertificate(certFile, keyFile string) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(certFile, keyFile)
}

// LoadCAPool loads a CA certificate pool from a PEM-encoded file.
func LoadCAPool(caFile string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("failed to parse CA certificate")
	}
	return pool, nil
}

// GenerateSelfSignedCert generates a self-signed TLS certificate for testing.
// Uses ECDSA P-256 for signing (ML-DSA integration pending Go stdlib support).
func GenerateSelfSignedCert(hosts []string, validFor time.Duration) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Ginger"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(validFor),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		DNSNames:              hosts,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}, nil
}

// OnRotationFunc is called when a certificate is rotated.
type OnRotationFunc func(cert tls.Certificate)

// KeyRotator watches certificate files and reloads them on change.
type KeyRotator struct {
	certFile string
	keyFile  string
	interval time.Duration

	mu       sync.RWMutex
	current  *tls.Certificate
	callback atomic.Value // stores OnRotationFunc
}

// NewKeyRotator creates a new KeyRotator.
func NewKeyRotator(certFile, keyFile string, interval time.Duration) (*KeyRotator, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("initial load: %w", err)
	}
	return &KeyRotator{
		certFile: certFile,
		keyFile:  keyFile,
		interval: interval,
		current:  &cert,
	}, nil
}

// GetCertificate returns the current certificate for use in tls.Config.GetCertificate.
func (r *KeyRotator) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current, nil
}

// OnRotation sets a callback that is called when the certificate is rotated.
func (r *KeyRotator) OnRotation(fn OnRotationFunc) {
	r.callback.Store(fn)
}

// Watch starts polling the certificate files. Blocks until context is cancelled.
func (r *KeyRotator) Watch(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.reload()
		}
	}
}

func (r *KeyRotator) reload() {
	cert, err := tls.LoadX509KeyPair(r.certFile, r.keyFile)
	if err != nil {
		return // keep current cert on error
	}

	r.mu.Lock()
	r.current = &cert
	r.mu.Unlock()

	if fn, ok := r.callback.Load().(OnRotationFunc); ok && fn != nil {
		fn(cert)
	}
}
