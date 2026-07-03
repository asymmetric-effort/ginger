# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| latest  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in Ginger, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

### How to Report

1. Email: security@asymmetric-effort.com
2. Include a detailed description of the vulnerability
3. Include steps to reproduce the issue
4. Include the potential impact

### Response Timeline

We follow these SLAs for vulnerability remediation:

| Severity | Response Time | Patch Time |
|----------|---------------|------------|
| Critical | 4 hours       | 24 hours   |
| High     | 12 hours      | 72 hours   |
| Medium   | 48 hours      | 30 days    |
| Low      | 5 days        | 90 days    |

### What to Expect

- Acknowledgment of your report within the response time above
- Regular updates on the progress of the fix
- Credit in the security advisory (unless you prefer to remain anonymous)
- Notification when the fix is released

## Security Standards

This project adheres to:

- OWASP Application Security Verification Standard (ASVS)
- CIS Benchmarks for Kubernetes
- NIST Post-Quantum Cryptography standards
- HIPAA, SOC 2, and GDPR compliance requirements

All network traffic is encrypted using TLS 1.3 with post-quantum cryptography (PQC).
All data at rest is encrypted using AES-256 or PQC-equivalent algorithms.

## Vulnerability Disclosure

Resolved vulnerabilities will be disclosed via GitHub Security Advisories after
patches are available.
