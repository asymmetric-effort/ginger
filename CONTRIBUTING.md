# Contributing to Ginger

Thank you for your interest in contributing to Ginger. This document provides
guidelines and instructions for contributing.

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md).

## Getting Started

### Prerequisites

- Go 1.26.3 or later
- Python 3.12 or later (for Python SDK)
- Node.js 22 or later (for TypeScript SDK)
- Docker (for integration and e2e tests)
- Git (configured with SSH, not HTTPS)

### Setup

1. Clone the repository using SSH:

```bash
git clone git@github.com:asymmetric-effort/ginger.git
cd ginger
```

2. Install git hooks:

```bash
ln -sf ../../git-hooks/pre-commit .git/hooks/pre-commit
ln -sf ../../git-hooks/pre-push .git/hooks/pre-push
```

3. Build the project:

```bash
go build ./...
```

4. Run tests:

```bash
go test ./...
```

## Development Workflow

### Branch Naming

Use descriptive branch names with a prefix:

- `feat/` for new features
- `fix/` for bug fixes
- `test/` for test additions or changes
- `docs/` for documentation changes
- `refactor/` for code refactoring
- `perf/` for performance improvements
- `chore/` for maintenance tasks

### Commit Messages

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/)
format:

```
feat: add adaptive sampling support
fix: correct PQC key rotation interval
test: add e2e tests for OTLP ingestion
docs: update API reference for query service
refactor: simplify trace storage pipeline
perf: optimize span batch encoding
chore: update CI pipeline configuration
```

### Code Standards

This project follows the [Asymmetric Effort Coding Standards](https://coding-standards.asymmetric-effort.com).

Key requirements:

- **Linting**: All code must pass strict linting. Linters run in pre-commit
  hooks and as the first CI stage.
- **Testing**: Minimum 98% code coverage. Tests must be organized into
  `unit/`, `integration/`, and `e2e/` directories.
- **No third-party dependencies** unless explicitly approved. This project
  prioritizes minimal dependencies to reduce supply chain attack surface.
- **No recursion** unless tail-call optimization is guaranteed.
- **All queues and buffers must be bounded.**
- **Pure functions are preferred** where possible.
- **No TODO or FIXME comments** left unresolved for the current task.

### Language-Specific Standards

#### Go

- Use `golangci-lint` with strict configuration
- No `interface{}` or `any` without documented justification
- All exported functions and types must have GoDoc comments

#### TypeScript

- Strict mode enabled, no suppressions
- `const` over `let`; `var` is never permitted
- Named exports only; default exports are not permitted
- No `any` types unless explicitly documented and justified

#### Python

- Type hints required for all function signatures
- Use `mypy` in strict mode
- Use `ruff` for linting and formatting

## Pull Request Process

1. Ensure all CI checks pass (lint, typecheck, test, build, e2e)
2. Ensure code coverage meets or exceeds 98%
3. Update documentation for any public API changes
4. Add entries to `CHANGELOG.md` for notable changes
5. Request review from at least one team member
6. Address all review comments before merging
7. Rebase on the latest target branch before merging

### PR Description

Every PR must include:

- **What**: A clear description of what changed
- **Why**: The motivation for the change
- **Testing**: How the change was tested
- **Security**: Any security considerations

## Security

All code reviews must include explicit security review for:

- Authentication and authorization
- Input validation and output encoding
- Data exposure and information leakage
- Cryptographic operations

See [SECURITY.md](SECURITY.md) for our security policy.

## Licensing

All contributions must be original work or covered by an MIT-compatible license.
By contributing, you agree that your contributions will be licensed under the
MIT License.
