# Development Guide

This guide provides information for developers working on the Hotel Reviews Microservice.

## Quick Start

### Prerequisites

- Go 1.22 or 1.23
- Docker and Docker Compose
- Git

### Setup Development Environment

```bash
# Clone the repository
git clone <repository-url>
cd hotel-reviews

# Setup development tools
make dev-setup

# Install dependencies
make deps

# Run quality checks
make quality-check
```

## Development Workflow

### 1. Code Quality Standards

We maintain high code quality standards with the following tools:

- **Formatting**: `gofmt` and `goimports`
- **Linting**: `golangci-lint` with comprehensive rules
- **Static Analysis**: `go vet` and `staticcheck`
- **Security**: `gosec` for security vulnerability scanning
- **Testing**: Minimum 42% coverage (target: 80%)

### 2. Pre-commit Hooks

Install pre-commit hooks to ensure quality before commits:

```bash
# Install pre-commit (if not already installed)
pip install pre-commit

# Install hooks
pre-commit install

# Run hooks manually
pre-commit run --all-files
```

### 3. Local Development Commands

Use the Makefile for common development tasks:

```bash
# Code quality
make fmt           # Format code
make lint          # Run linters
make vet           # Run go vet
make security      # Security scan

# Testing
make test          # Run unit tests
make test-race     # Run tests with race detection
make test-coverage # Run tests with coverage
make test-integration # Run integration tests

# Build and run
make build         # Build the application
make docker-build  # Build Docker image
make docker-run    # Run in Docker

# Combined checks
make quality-check # Run all quality checks
make ci-local      # Run full CI pipeline locally
```

### 4. Testing Guidelines

#### Unit Tests
- Place test files next to source files with `_test.go` suffix
- Use table-driven tests for multiple test cases
- Mock external dependencies
- Aim for comprehensive error handling coverage

#### Integration Tests
- Use build tag `//go:build integration`
- Test complete workflows with real dependencies
- Use TestContainers for database/Redis testing

#### Test Coverage
- Current baseline: 42%
- Target: 80%
- Coverage must not decrease below baseline
- Focus on testing:
  - Error handling paths
  - Business logic edge cases
  - Integration scenarios

### 5. Code Organization

```
internal/
├── application/     # Application layer (handlers, middleware)
├── domain/         # Domain layer (entities, services, interfaces)
└── infrastructure/ # Infrastructure layer (database, external services)

pkg/               # Shared packages
tests/             # Test utilities and integration tests
cmd/               # Application entry points
```

### 6. Git Workflow

1. Create feature branch from `develop`
2. Make small, focused commits
3. Ensure all quality gates pass locally
4. Open pull request to `develop`
5. Address review feedback
6. Merge after approval and CI passes

### 7. Quality Gates

Every pull request must pass:

✅ **Code Formatting** - All files properly formatted  
✅ **Static Analysis** - No issues from `go vet`  
✅ **Linting** - All `golangci-lint` checks pass  
✅ **Test Coverage** - Meets minimum threshold (42%)  
✅ **Security Scan** - No high-severity vulnerabilities  
✅ **Unit Tests** - All tests pass with race detection  
✅ **Integration Tests** - All integration tests pass  

### 8. Performance Considerations

- Run benchmarks for performance-critical code
- Monitor memory allocations
- Use profiling tools for optimization
- Consider caching strategies

### 9. Documentation

- Update API documentation for interface changes
- Add inline comments for complex business logic
- Update README for significant changes
- Create ADRs for architectural decisions

### 10. Troubleshooting

#### Common Issues

**Coverage Below Threshold**
```bash
# Check current coverage
make test-coverage

# Identify untested code
go tool cover -html=coverage.html
```

**Linting Failures**
```bash
# Run specific linter
golangci-lint run --config=.golangci.yml

# Auto-fix some issues
golangci-lint run --fix
```

**Build Failures**
```bash
# Clean and rebuild
make clean
make build

# Check dependencies
go mod verify
go mod tidy
```

### 11. IDE Setup

#### VS Code
Recommended extensions:
- Go (official Go extension)
- golangci-lint
- Test Explorer for Go
- Coverage Gutters

#### GoLand/IntelliJ
- Enable Go modules support
- Configure golangci-lint integration
- Set up test coverage highlighting

### 12. CI/CD Integration

The project uses GitHub Actions with multiple quality gates:

- **Quality Gate Workflow**: Runs on every PR
- **Main CI Pipeline**: Comprehensive testing and deployment
- **Security Scanning**: Automated vulnerability detection
- **Performance Benchmarks**: Performance regression detection

## Getting Help

- Check existing issues and documentation
- Run `make help` for available commands
- Review code comments and tests for examples
- Contact the development team for architecture questions