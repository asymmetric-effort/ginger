# Post-Quantum Cryptography

Ginger encrypts all network communication using TLS 1.3 with hybrid
post-quantum key exchange. This document explains what that means, why it
matters, how ginger implements it, and what the current limitations are.

## The Problem

Traditional TLS key exchange algorithms (RSA, ECDH) rely on the computational
difficulty of factoring large integers or solving the elliptic curve discrete
logarithm problem. A sufficiently powerful quantum computer running Shor's
algorithm could break both in polynomial time.

An attacker performing "harvest now, decrypt later" can record encrypted
traffic today and decrypt it years from now when quantum computers become
available. For tracing data that contains service topology, latency profiles,
error rates, and potentially sensitive span attributes, this represents a
real long-term confidentiality risk.

## The Solution: Hybrid PQC Key Exchange

Ginger uses **X25519+ML-KEM-768**, a hybrid key exchange that combines:

- **X25519**: A classical elliptic curve Diffie-Hellman key exchange that is
  well-understood, battle-tested, and secure against all known classical attacks.
- **ML-KEM-768** (formerly Kyber-768): A lattice-based key encapsulation
  mechanism standardized by NIST as FIPS 203, designed to resist attacks from
  both classical and quantum computers.

The hybrid approach means that the connection is secure as long as *either*
algorithm remains unbroken. If ML-KEM is later found to have a flaw, the
X25519 component still provides classical security. If quantum computers break
X25519, the ML-KEM component provides post-quantum security.

## NIST Standards Compliance

Ginger's PQC implementation aligns with the following NIST standards:

| Standard | Algorithm | Ginger Usage |
|----------|-----------|--------------|
| [FIPS 203](https://doi.org/10.6028/NIST.FIPS.203) | ML-KEM (Module-Lattice-Based Key-Encapsulation Mechanism) | Key exchange via X25519MLKEM768 hybrid |
| TLS 1.3 (RFC 8446) | Mandatory cipher suites | Minimum TLS version enforced |

ML-KEM-768 provides NIST Security Level 3, roughly equivalent to AES-192.
This exceeds the Level 1 (AES-128 equivalent) minimum and provides a security
margin against advances in both classical and quantum cryptanalysis.

## Implementation

Ginger's PQC support is implemented in `internal/crypto/pqc/tls.go` using
Go's standard library `crypto/tls` package. Go 1.24 and later include native
support for the X25519+ML-KEM-768 hybrid key exchange, registered as the
`tls.X25519MLKEM768` curve identifier.

### Core Configuration

The `NewTLSConfig` function creates a TLS configuration with PQC-capable
cipher suites:

```go
// internal/crypto/pqc/tls.go, line 35
func NewTLSConfig(mode TLSMode) *tls.Config {
    cfg := &tls.Config{
        MinVersion: tls.VersionTLS13,
        CurvePreferences: []tls.CurveID{
            tls.X25519MLKEM768, // Hybrid PQC: X25519 + ML-KEM-768
            tls.X25519,         // Fallback: classical ECDH
            tls.CurveP256,      // Fallback: NIST P-256
        },
    }
    // ...
}
```

The `CurvePreferences` order is significant. The TLS handshake attempts
X25519MLKEM768 first. If the client does not support it (older TLS stacks),
the server falls back to X25519, then P-256. This provides backward
compatibility while preferring PQC when both sides support it.

### What Happens During a TLS Handshake

When a client connects to ginger with PQC support:

1. **ClientHello**: The client advertises supported key exchange groups,
   including X25519MLKEM768 (code point 0x11EC).
2. **ServerHello**: The server selects X25519MLKEM768 from the client's
   offered groups.
3. **Key Exchange**: Both sides perform a hybrid key exchange:
   - An X25519 ECDH exchange produces a 32-byte shared secret.
   - An ML-KEM-768 encapsulation produces a second 32-byte shared secret.
   - Both secrets are combined (concatenated and hashed) to derive the
     TLS session keys.
4. **Encrypted Communication**: All subsequent data is encrypted with
   AES-256-GCM or ChaCha20-Poly1305 using the derived keys.

The entire exchange adds approximately 1,100 bytes to the handshake
(the ML-KEM-768 public key is 1,184 bytes and the ciphertext is 1,088 bytes)
compared to a classical X25519-only handshake. The computational overhead
is negligible on modern hardware.

### Server TLS Configuration

To enable PQC on the ginger server:

```yaml
# config.yaml
tls:
  enabled: true
  cert_file: /etc/ginger/tls/server.crt
  key_file: /etc/ginger/tls/server.key
  ca_file: /etc/ginger/tls/ca.crt    # For mTLS
  client_auth: require                 # none | request | require
```

The server code loads the configuration and creates a PQC-enabled TLS
listener:

```go
// Using the pqc package
cert, _ := pqc.LoadCertificate("/etc/ginger/tls/server.crt", "/etc/ginger/tls/server.key")
tlsCfg := pqc.NewServerTLSConfig(cert)

// All connections now negotiate X25519+ML-KEM-768 when possible
listener := tls.NewListener(tcpListener, tlsCfg)
```

### Mutual TLS (mTLS)

For component-to-component communication (collector to query service,
collector to storage), ginger supports mutual TLS where both sides
present certificates:

```go
// internal/crypto/pqc/tls.go, line 57
func NewMutualTLSConfig(caPool *x509.CertPool, cert tls.Certificate) *tls.Config {
    cfg := NewTLSConfig(TLSModeMutual)
    cfg.Certificates = []tls.Certificate{cert}
    cfg.ClientCAs = caPool
    cfg.RootCAs = caPool
    return cfg
}
```

Both the client certificate verification and the key exchange use the
PQC-capable configuration. The CA pool validates that the peer's certificate
was issued by a trusted authority.

### Client TLS Configuration

SDK clients connecting to ginger also negotiate PQC:

```go
// Using the pqc package for a client connection
caPool, _ := pqc.LoadCAPool("/etc/ginger/tls/ca.crt")
tlsCfg := pqc.NewClientTLSConfig(caPool)

// This connection will use X25519+ML-KEM-768
conn, _ := tls.Dial("tcp", "ginger:4317", tlsCfg)
```

### Certificate Rotation

The `KeyRotator` watches certificate files and reloads them without
restarting the server:

```go
// internal/crypto/pqc/tls.go, line 149
rotator, _ := pqc.NewKeyRotator(
    "/etc/ginger/tls/server.crt",
    "/etc/ginger/tls/server.key",
    5 * time.Minute,  // Poll interval
)

// Use rotator.GetCertificate as the TLS config callback
tlsCfg.GetCertificate = rotator.GetCertificate

// Optional: get notified on rotation
rotator.OnRotation(func(cert tls.Certificate) {
    log.Info("certificate rotated")
})

// Start watching in the background
go rotator.Watch(ctx)
```

This enables zero-downtime certificate rotation, which is essential for
environments with short-lived certificates (cert-manager, Vault PKI).

## Where PQC Is Applied

| Communication Path | Protocol | PQC Applied |
|--------------------|----------|-------------|
| Client SDK to Collector | OTLP gRPC (4317) | Yes, via TLS 1.3 |
| Client SDK to Collector | OTLP HTTP (4318) | Yes, via TLS 1.3 |
| Jaeger agent to Collector | Thrift HTTP (14268) | Yes, via TLS 1.3 |
| Jaeger agent to Collector | Thrift UDP (6831/6832) | No (UDP, no TLS) |
| Collector to InfluxDB | HTTP API | Yes, via TLS 1.3 |
| Query Service to clients | HTTP REST (16686) | Yes, via TLS 1.3 |
| Component to component | gRPC (mTLS) | Yes, via TLS 1.3 |

UDP receivers (Jaeger Thrift Compact/Binary on ports 6831/6832) do not use
TLS because UDP does not support it. In high-security environments, use the
HTTP or gRPC receivers instead, or run the UDP receivers on a trusted network
segment with network-level encryption (WireGuard, IPsec).

## Current Limitations

### Signing Algorithms

The TLS key exchange uses hybrid PQC (X25519+ML-KEM-768), but the
certificate signing currently uses ECDSA P-256. ML-DSA (FIPS 204, formerly
Dilithium) for post-quantum digital signatures is not yet available in Go's
standard library.

When Go adds `crypto/mldsa` support, ginger will integrate ML-DSA-65 for
certificate signing to provide full post-quantum protection for both key
exchange and authentication.

```go
// internal/crypto/pqc/tls.go, line 98
// GenerateSelfSignedCert generates a self-signed TLS certificate for testing.
// Uses ECDSA P-256 for signing (ML-DSA integration pending Go stdlib support).
```

### Client Compatibility

Not all TLS clients support X25519MLKEM768 yet. Ginger's `CurvePreferences`
includes X25519 and P-256 as fallbacks, so connections from older clients
succeed but use classical key exchange. The server logs which key exchange
was negotiated, allowing operators to identify clients that have not upgraded.

### Data at Rest

PQC applies to data in transit (TLS connections). Data at rest in InfluxDB
is encrypted using InfluxDB's own encryption mechanisms (typically AES-256),
which is a symmetric algorithm not vulnerable to quantum attacks.

## Verifying PQC Is Active

To verify that a connection is using PQC, check the TLS connection state:

```go
conn := resp.TLS
fmt.Printf("TLS version: %x\n", conn.Version)           // 0x0304 = TLS 1.3
fmt.Printf("Cipher suite: %x\n", conn.CipherSuite)       // AES-256-GCM or ChaCha20
fmt.Printf("Key exchange: %s\n", conn.NegotiatedProtocol) // h2
```

With OpenSSL 3.5+ or a PQC-aware client:

```bash
openssl s_client -connect ginger:4317 -groups X25519MLKEM768
# Look for "Server Temp Key: X25519MLKEM768" in the output
```

## References

- [NIST FIPS 203: ML-KEM](https://doi.org/10.6028/NIST.FIPS.203) -- Module-Lattice-Based Key-Encapsulation Mechanism Standard
- [NIST FIPS 204: ML-DSA](https://doi.org/10.6028/NIST.FIPS.204) -- Module-Lattice-Based Digital Signature Standard (future integration)
- [Go crypto/mlkem package](https://pkg.go.dev/crypto/mlkem) -- Go standard library ML-KEM implementation
- [RFC 8446: TLS 1.3](https://datatracker.ietf.org/doc/html/rfc8446) -- Transport Layer Security version 1.3
- [Go crypto/tls X25519MLKEM768](https://pkg.go.dev/crypto/tls#CurveID) -- Hybrid PQC curve identifier in Go
- Source code: [`internal/crypto/pqc/tls.go`](https://github.com/asymmetric-effort/ginger/blob/main/internal/crypto/pqc/tls.go)
