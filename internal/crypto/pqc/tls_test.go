package pqc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewTLSConfigServer(t *testing.T) {
	cfg := NewTLSConfig(TLSModeServer)
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Error("MinVersion should be TLS 1.3")
	}
	if len(cfg.CurvePreferences) != 3 {
		t.Errorf("CurvePreferences = %d, want 3", len(cfg.CurvePreferences))
	}
	if cfg.CurvePreferences[0] != tls.X25519MLKEM768 {
		t.Error("first curve should be X25519MLKEM768")
	}
}

func TestNewTLSConfigClient(t *testing.T) {
	cfg := NewTLSConfig(TLSModeClient)
	if cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false")
	}
}

func TestNewTLSConfigMutual(t *testing.T) {
	cfg := NewTLSConfig(TLSModeMutual)
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Error("ClientAuth should be RequireAndVerifyClientCert")
	}
}

func TestNewMutualTLSConfig(t *testing.T) {
	cert, err := GenerateSelfSignedCert([]string{"localhost"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	cfg := NewMutualTLSConfig(pool, cert)
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Error("mTLS ClientAuth")
	}
	if cfg.ClientCAs != pool {
		t.Error("mTLS ClientCAs")
	}
	if cfg.RootCAs != pool {
		t.Error("mTLS RootCAs")
	}
	if len(cfg.Certificates) != 1 {
		t.Error("mTLS Certificates")
	}
}

func TestNewServerTLSConfig(t *testing.T) {
	cert, err := GenerateSelfSignedCert([]string{"localhost"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewServerTLSConfig(cert)
	if len(cfg.Certificates) != 1 {
		t.Error("server should have 1 cert")
	}
}

func TestNewClientTLSConfig(t *testing.T) {
	pool := x509.NewCertPool()
	cfg := NewClientTLSConfig(pool)
	if cfg.RootCAs != pool {
		t.Error("client RootCAs")
	}
}

func TestGenerateSelfSignedCert(t *testing.T) {
	cert, err := GenerateSelfSignedCert([]string{"localhost", "ginger.local"}, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) != 1 {
		t.Error("should have 1 cert")
	}
	if cert.PrivateKey == nil {
		t.Error("should have private key")
	}

	// Parse and verify
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Subject.Organization[0] != "Ginger" {
		t.Error("org")
	}
	if len(parsed.DNSNames) != 2 {
		t.Errorf("DNS names = %v", parsed.DNSNames)
	}
}

func TestTLSHandshake(t *testing.T) {
	// Generate self-signed cert
	cert, err := GenerateSelfSignedCert([]string{"localhost"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Parse the cert to add to CA pool
	parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(parsedCert)

	serverCfg := NewServerTLSConfig(cert)
	clientCfg := NewClientTLSConfig(pool)

	// Start TLS server
	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		buf := make([]byte, 5)
		n, err := conn.Read(buf)
		if err != nil {
			done <- err
			return
		}
		_, err = conn.Write(buf[:n])
		done <- err
	}()

	// Connect client — use localhost hostname for cert validation
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	conn, err := tls.Dial("tcp", "localhost:"+port, clientCfg)
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	defer conn.Close()

	// Verify PQC key exchange was available
	state := conn.ConnectionState()
	if state.Version != tls.VersionTLS13 {
		t.Errorf("TLS version = %d, want 1.3", state.Version)
	}

	// Send/receive data
	_, err = conn.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	_, err = conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf) != "hello" {
		t.Errorf("echo = %q", buf)
	}

	if err := <-done; err != nil {
		t.Fatalf("server: %v", err)
	}
}

func TestLoadCertificate(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeTempCert(t, dir)

	cert, err := LoadCertificate(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(cert.Certificate) == 0 {
		t.Error("should load cert")
	}
}

func TestLoadCertificateInvalid(t *testing.T) {
	_, err := LoadCertificate("/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Error("should fail for nonexistent files")
	}
}

func TestLoadCAPool(t *testing.T) {
	dir := t.TempDir()
	certFile, _ := writeTempCert(t, dir)

	pool, err := LoadCAPool(certFile)
	if err != nil {
		t.Fatal(err)
	}
	if pool == nil {
		t.Error("pool should not be nil")
	}
}

func TestLoadCAPoolInvalidFile(t *testing.T) {
	_, err := LoadCAPool("/nonexistent/ca.pem")
	if err == nil {
		t.Error("should fail for nonexistent file")
	}
}

func TestLoadCAPoolInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "bad.pem")
	os.WriteFile(f, []byte("not a pem"), 0644)
	_, err := LoadCAPool(f)
	if err == nil {
		t.Error("should fail for invalid PEM")
	}
}

func TestKeyRotator(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeTempCert(t, dir)

	rotator, err := NewKeyRotator(certFile, keyFile, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	// Get initial cert
	cert, err := rotator.GetCertificate(nil)
	if err != nil || cert == nil {
		t.Fatal("initial cert")
	}

	// Set callback
	var rotated atomic.Int32
	rotator.OnRotation(func(c tls.Certificate) {
		rotated.Add(1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	go rotator.Watch(ctx)

	// Overwrite cert file (trigger rotation)
	time.Sleep(100 * time.Millisecond)
	writeTempCertToFiles(t, certFile, keyFile)

	time.Sleep(200 * time.Millisecond)

	if rotated.Load() == 0 {
		t.Error("rotation callback should have fired")
	}
}

func TestKeyRotatorInvalidInit(t *testing.T) {
	_, err := NewKeyRotator("/nonexistent/cert.pem", "/nonexistent/key.pem", time.Minute)
	if err == nil {
		t.Error("should fail for nonexistent files")
	}
}

func TestKeyRotatorBadReload(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeTempCert(t, dir)

	rotator, err := NewKeyRotator(certFile, keyFile, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the cert file
	os.WriteFile(certFile, []byte("corrupted"), 0644)

	// Reload should silently keep old cert
	rotator.reload()

	cert, err := rotator.GetCertificate(nil)
	if err != nil || cert == nil {
		t.Error("should still have old cert after bad reload")
	}
}

func TestKeyRotatorNoCallback(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := writeTempCert(t, dir)

	rotator, err := NewKeyRotator(certFile, keyFile, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	// Reload without setting callback — should not panic
	rotator.reload()
}

// Helper to generate and write test certs.

func writeTempCert(t *testing.T, dir string) (certFile, keyFile string) {
	t.Helper()
	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	writeTempCertToFiles(t, certFile, keyFile)
	return
}

func writeTempCertToFiles(t *testing.T, certFile, keyFile string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{Organization: []string{"Test"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	os.WriteFile(certFile, certPEM, 0644)
	os.WriteFile(keyFile, keyPEM, 0600)
}
