# Contributing to Hotel Reviews Microservice

We appreciate your interest in contributing to the Hotel Reviews Microservice! This document provides guidelines and instructions for contributing to the project.

## 🎯 **Code of Conduct**

We are committed to providing a welcoming and inspiring community for all. Please read our Code of Conduct before participating.

## 🚀 **Getting Started**

### Prerequisites

- Go 1.23 or higher
- Docker and Docker Compose
- Git
- Basic understanding of microservices and Clean Architecture

### Development Environment Setup

1. **Fork and Clone**
```bash
git clone https://github.com/YOUR_USERNAME/hotel-reviews-microservice.git
cd hotel-reviews-microservice
```

2. **Install Dependencies**
```bash
go mod download
```

3. **Start Development Environment**
```bash
docker-compose up -d postgres redis kafka
```

4. **Run Tests**
```bash
make test
```

5. **Start the Application**
```bash
make run
```

## 📋 **Development Workflow**

### Branch Strategy

- `main` - Production-ready code
- `develop` - Integration branch for features
- `feature/*` - Feature development branches
- `hotfix/*` - Critical bug fixes
- `release/*` - Release preparation branches

### Workflow Steps

1. **Create Feature Branch**
```bash
git checkout -b feature/your-feature-name
```

2. **Make Changes**
   - Follow coding standards
   - Add tests for new functionality
   - Update documentation if needed

3. **Test Your Changes**
```bash
make test
make lint
make security-check
```

4. **Commit Changes**
```bash
git add .
git commit -m "feat: add amazing new feature"
```

5. **Push and Create PR**
```bash
git push origin feature/your-feature-name
```

## 📝 **Commit Convention**

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

### Format
```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

### Types
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

### Examples
```bash
feat(api): add hotel search endpoint with filtering
fix(cache): resolve Redis connection timeout issue
docs(readme): update API documentation examples
test(domain): add unit tests for review entity
```

## 🏗️ **Architecture Guidelines**

### Clean Architecture Principles

1. **Dependency Direction**: All dependencies point inward toward the domain
2. **Layer Separation**: Clear boundaries between layers
3. **Interface Segregation**: Small, focused interfaces
4. **Single Responsibility**: One reason to change per component

### Layer Responsibilities

#### Domain Layer (`internal/domain/`)
- Business entities and value objects
- Domain services and business rules
- Repository interfaces
- Domain events

#### Application Layer (`internal/application/`)
- Use cases and business workflows
- HTTP handlers and controllers
- Application services
- External service interfaces

#### Infrastructure Layer (`internal/infrastructure/`)
- Database implementations
- External service clients
- Messaging and caching
- Configuration and logging

## 📊 **Testing Guidelines**

### Test Categories

1. **Unit Tests** - Test individual components in isolation
2. **Integration Tests** - Test component interactions
3. **End-to-End Tests** - Test complete user workflows
4. **Performance Tests** - Test system performance and scalability

### Testing Standards

- **Coverage Target**: 80% code coverage minimum
- **Test Naming**: `TestComponent_Method_Scenario`
- **Test Structure**: Arrange, Act, Assert (AAA pattern)
- **Mocking**: Use interfaces for external dependencies

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific test package
go test ./internal/domain/...

# Run integration tests
make test-integration

# Run load tests
make test-load
```

## 🔒 **Security Guidelines**

### Security Requirements

1. **Input Validation**: Validate all inputs at boundaries
2. **Authentication**: Implement proper JWT handling
3. **Authorization**: Enforce RBAC consistently
4. **Data Protection**: Encrypt sensitive data
5. **Error Handling**: Don't expose internal details

### Security Checklist

- [ ] Input validation and sanitization
- [ ] SQL injection prevention
- [ ] XSS protection
- [ ] CSRF protection
- [ ] Rate limiting implementation
- [ ] Secure password handling
- [ ] Proper error messages

## 📚 **Documentation Standards**

### Required Documentation

1. **API Documentation**: OpenAPI/Swagger specs
2. **Architecture Decisions**: ADR documents
3. **Code Comments**: Public interfaces and complex logic
4. **README Updates**: Feature documentation
5. **Migration Guides**: Breaking changes

### Documentation Guidelines

- Write clear, concise documentation
- Include code examples
- Keep documentation in sync with code
- Use proper markdown formatting

## 🧪 **Code Quality Standards**

### Quality Gates

All contributions must pass these quality gates:

1. **Linting**: `golangci-lint run`
2. **Testing**: Minimum 80% coverage
3. **Security**: `gosec` security scan
4. **Dependencies**: `go mod tidy` and vulnerability check
5. **Build**: Successful compilation

### Code Style

- Follow Go formatting standards (`gofmt`)
- Use meaningful variable and function names
- Keep functions small and focused
- Write self-documenting code
- Use interfaces for dependencies

### Pre-commit Hooks

Install pre-commit hooks:

```bash
make install-hooks
```

This will run:
- Code formatting
- Linting
- Security checks
- Unit tests

## 🔧 **Development Tools**

### Required Tools

```bash
# Install development tools
make install-tools
```

This installs:
- `golangci-lint` - Code linting
- `gosec` - Security analysis
- `migrate` - Database migrations
- `mockgen` - Mock generation
- `swag` - API documentation

### Useful Make Targets

```bash
make help                 # Show all available targets
make build               # Build the application
make run                 # Run the application
make test                # Run tests
make test-coverage       # Run tests with coverage
make lint                # Run linter
make security-check      # Run security analysis
make docker-build        # Build Docker image
make docker-run          # Run in Docker
make migrate-up          # Run database migrations
make migrate-down        # Rollback migrations
make generate            # Generate code (mocks, swagger)
make clean               # Clean build artifacts
```

## 🐛 **Issue Reporting**

### Bug Reports

When reporting bugs, please include:

1. **Description**: Clear description of the issue
2. **Steps to Reproduce**: Detailed reproduction steps
3. **Expected Behavior**: What should happen
4. **Actual Behavior**: What actually happens
5. **Environment**: OS, Go version, etc.
6. **Logs**: Relevant log output
7. **Screenshots**: If applicable

### Feature Requests

For feature requests, please include:

1. **Problem Statement**: What problem does this solve?
2. **Proposed Solution**: Detailed feature description
3. **Alternatives**: Other solutions considered
4. **Implementation Notes**: Technical considerations
5. **Breaking Changes**: Any compatibility concerns

## 📋 **Pull Request Process**

### PR Requirements

1. **Description**: Clear description of changes
2. **Issue Reference**: Link to related issue
3. **Testing**: Evidence of testing
4. **Documentation**: Updated documentation
5. **Breaking Changes**: Clear indication of breaking changes

### PR Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] Tests added/updated
- [ ] No breaking changes (or documented)
```

### Review Process

1. **Automated Checks**: CI pipeline must pass
2. **Code Review**: At least one approval required
3. **Documentation**: Review documentation updates
4. **Testing**: Verify test coverage and quality
5. **Security**: Check for security implications

## 🎯 **Contribution Types**

### Code Contributions

- Bug fixes
- New features
- Performance improvements
- Refactoring
- Test improvements

### Documentation Contributions

- API documentation
- Tutorials and guides
- Code examples
- Architecture documentation
- Translation

### Community Contributions

- Issue triaging
- Code reviews
- Mentoring newcomers
- Blog posts and articles
- Conference talks

## 📞 **Getting Help**

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: General questions and ideas
- **Email**: gkbiswas@gmail.com for sensitive issues

### Response Times

- **Bug Reports**: 24-48 hours
- **Feature Requests**: 3-5 days
- **Pull Requests**: 2-3 days
- **Security Issues**: 24 hours

## 🏆 **Recognition**

Contributors will be recognized in:

- **CONTRIBUTORS.md**: List of all contributors
- **Release Notes**: Major contributions highlighted
- **Documentation**: Author attribution
- **Community**: Social media recognition

## 📄 **Legal**

By contributing to this project, you agree that your contributions will be licensed under the MIT License.

---

**Thank you for contributing to the Hotel Reviews Microservice! 🙏**

Your contributions help make this project better for everyone.