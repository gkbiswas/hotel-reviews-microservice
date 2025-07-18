# CI/CD Pipeline Guide

This document provides a comprehensive guide for the GitHub Actions CI/CD pipeline implemented for the Hotel Reviews Microservice.

## Overview

The CI/CD pipeline is designed to ensure code quality, security, and reliability through automated testing, building, and deployment processes. The pipeline includes:

- **Code Quality Checks**: golangci-lint with custom configuration
- **Security Scanning**: gosec, Snyk, Trivy, and other security tools
- **Testing**: Unit tests, integration tests, and load tests
- **Docker Build**: Multi-stage builds with security scanning
- **Deployment**: Automated deployment to staging and production environments
- **Monitoring**: Performance benchmarks and health checks
- **Notifications**: Slack, email, and other notification channels

## Pipeline Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Code Quality  │    │   Security      │    │   Unit Tests    │
│   - golangci    │    │   - gosec       │    │   - Go test     │
│   - staticcheck │    │   - Snyk        │    │   - Coverage    │
│   - gosec       │    │   - Trivy       │    │   - Race det.   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Integration    │
                    │  Tests          │
                    │  - TestContainers│
                    │  - DB Tests     │
                    │  - API Tests    │
                    └─────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Docker Build   │
                    │  - Multi-stage  │
                    │  - Security     │
                    │  - Push to Reg  │
                    └─────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Load Testing   │
                    │  - K6 Tests     │
                    │  - Performance  │
                    │  - Benchmarks   │
                    └─────────────────┘
                                 │
                    ┌─────────────────┐
                    │  Deployment     │
                    │  - Staging      │
                    │  - Production   │
                    │  - Health Check │
                    └─────────────────┘
```

## Workflows

### 1. Main CI/CD Pipeline (`ci.yml`)

The main pipeline runs on every push and pull request to main branches.

**Triggers:**
- Push to `main`, `develop`, or `feature/*` branches
- Pull requests to `main` or `develop`
- Manual workflow dispatch

**Jobs:**
- `detect-changes`: Detects file changes to optimize pipeline execution
- `code-quality`: Runs linting and static analysis
- `unit-tests`: Executes unit tests with coverage reporting
- `integration-tests`: Runs integration tests with TestContainers
- `benchmarks`: Performance benchmarking
- `docker-build`: Builds and pushes Docker images
- `load-testing`: Runs load tests with K6
- `deploy-staging`: Deploys to staging environment
- `deploy-production`: Deploys to production environment
- `notify`: Sends notifications to configured channels

### 2. Security Scanning (`security-scan.yml`)

Dedicated security scanning workflow that can be run independently.

**Features:**
- Code security scanning with gosec and Semgrep
- Dependency vulnerability scanning with govulncheck and Snyk
- Secret scanning with TruffleHog and GitLeaks
- Docker security scanning with Hadolint and Trivy
- Compliance checks with Checkov

### 3. Docker Build (`docker-build.yml`)

Reusable workflow for building and pushing Docker images.

**Features:**
- Multi-platform builds (AMD64, ARM64)
- Security scanning with Trivy and Snyk
- Image signing with Cosign
- Layer analysis and optimization
- Automatic cleanup of old images

### 4. Deployment (`deploy.yml`)

Flexible deployment workflow supporting multiple strategies.

**Deployment Strategies:**
- **Rolling Update**: Gradual replacement of old instances
- **Blue-Green**: Zero-downtime deployment with instant switchover
- **Canary**: Gradual traffic shift to new version

**Features:**
- Health checks and smoke tests
- Automatic rollback on failure
- Environment-specific configurations
- Approval workflows for production

### 5. Performance Benchmarks (`performance-benchmarks.yml`)

Comprehensive performance testing and benchmarking.

**Test Types:**
- Unit benchmarks with memory/CPU profiling
- Integration benchmarks with real services
- Load testing with K6
- Memory leak detection

### 6. Notifications (`notifications.yml`)

Centralized notification system for all pipeline events.

**Supported Channels:**
- Slack
- Discord
- Microsoft Teams
- Email
- Telegram
- PagerDuty
- Datadog

## Configuration

### Required Secrets

Configure the following secrets in your GitHub repository:

#### Docker Registry
```
DOCKER_USERNAME=your-docker-username
DOCKER_PASSWORD=your-docker-password
```

#### AWS (for ECR and ECS deployment)
```
AWS_ACCESS_KEY_ID=your-aws-access-key
AWS_SECRET_ACCESS_KEY=your-aws-secret-key
AWS_REGION=us-east-1
AWS_ACCOUNT_ID=123456789012
ECS_CLUSTER=your-ecs-cluster
ECS_SERVICE=your-ecs-service
ECS_TASK_DEFINITION=your-task-definition
```

#### Notification Services
```
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/...
TEAMS_WEBHOOK_URL=https://outlook.office.com/webhook/...
NOTIFICATION_EMAIL=alerts@yourcompany.com
EMAIL_USERNAME=smtp-username
EMAIL_PASSWORD=smtp-password
```

#### Security Tools
```
SNYK_TOKEN=your-snyk-token
CODECOV_TOKEN=your-codecov-token
SONAR_TOKEN=your-sonar-token
```

#### Monitoring
```
DATADOG_API_KEY=your-datadog-api-key
PAGERDUTY_INTEGRATION_KEY=your-pagerduty-key
```

### Environment Variables

Environment-specific configurations are stored in:
- `.github/environments/staging.yml`
- `.github/environments/production.yml`

### Code Quality Configuration

The pipeline uses `.golangci.yml` for comprehensive code quality checks:

```yaml
# Key linters enabled:
- gosec        # Security issues
- govet        # Go vet analysis
- staticcheck  # Advanced static analysis
- errcheck     # Unchecked errors
- gosimple     # Simplification suggestions
- ineffassign  # Ineffectual assignments
- misspell     # Misspelled words
- gocyclo      # Cyclomatic complexity
- goconst      # Repeated strings
- goimports    # Import formatting
```

## Usage

### Running the Pipeline

1. **Automatic Triggers**: The pipeline runs automatically on:
   - Push to protected branches
   - Pull request creation/updates
   - Scheduled runs (security scans, benchmarks)

2. **Manual Triggers**: Use workflow dispatch to run specific workflows:
   ```bash
   # Using GitHub CLI
   gh workflow run ci.yml
   gh workflow run security-scan.yml --ref main
   gh workflow run deploy.yml -f environment=staging -f image_tag=latest
   ```

### Monitoring Pipeline Status

1. **GitHub Actions Tab**: View workflow runs and logs
2. **Slack Notifications**: Receive real-time updates
3. **Email Alerts**: Get notified of failures
4. **Datadog**: Monitor deployment metrics

### Deployment Process

#### Staging Deployment
1. Push code to `develop` branch
2. Pipeline runs automatically
3. After successful tests, deploys to staging
4. Health checks verify deployment

#### Production Deployment
1. Create pull request to `main`
2. Code review and approval
3. Merge to `main`
4. Pipeline runs with production deployment
5. Manual approval required for production (if configured)
6. Blue-green deployment to production
7. Health checks and smoke tests
8. Automatic rollback on failure

### Load Testing

Load tests run automatically on deployments and can be triggered manually:

```javascript
// Basic load test configuration
export const options = {
  stages: [
    { duration: '30s', target: 10 },  // Ramp up
    { duration: '60s', target: 10 },  // Sustained load
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.05'],
  },
};
```

### Security Scanning

Security scans run on every push and daily via scheduled jobs:

- **SAST**: Static Application Security Testing
- **Dependency Scanning**: Vulnerability checks for dependencies
- **Secret Scanning**: Detect leaked credentials
- **Container Scanning**: Docker image vulnerabilities
- **Compliance Checks**: Policy and compliance validation

## Troubleshooting

### Common Issues

1. **Pipeline Failures**
   - Check the GitHub Actions logs
   - Review the specific job that failed
   - Look for error messages in the workflow summary

2. **Test Failures**
   - Unit tests: Check for race conditions or environment issues
   - Integration tests: Verify service dependencies are available
   - Load tests: Ensure the application is running and accessible

3. **Security Issues**
   - Review security scan results
   - Check for known vulnerabilities in dependencies
   - Verify secrets are properly configured

4. **Deployment Failures**
   - Check health endpoints
   - Verify environment configuration
   - Review rollback logs if automatic rollback occurred

### Debug Mode

Enable debug logging by setting repository variables:
```
ACTIONS_STEP_DEBUG=true
ACTIONS_RUNNER_DEBUG=true
```

### Pipeline Optimization

1. **Caching**: The pipeline uses aggressive caching for:
   - Go module dependencies
   - Docker layers
   - Build artifacts

2. **Parallel Execution**: Jobs run in parallel where possible:
   - Code quality checks
   - Security scans
   - Testing phases

3. **Conditional Execution**: Jobs skip when not needed:
   - No Go files changed
   - No Docker files changed
   - Manual skip flags

## Best Practices

### Code Quality
- Maintain > 80% test coverage
- Address all linting issues
- Follow Go best practices
- Keep functions small and focused

### Security
- Regularly update dependencies
- Monitor security advisories
- Use least privilege for secrets
- Rotate credentials regularly

### Deployment
- Use feature flags for risky changes
- Monitor deployments closely
- Have rollback procedures ready
- Test in staging first

### Performance
- Monitor key metrics
- Set SLA thresholds
- Run regular load tests
- Profile memory usage

## Metrics and Monitoring

### Key Metrics Tracked
- Build success rate
- Test coverage percentage
- Security vulnerabilities
- Deployment frequency
- Lead time for changes
- Mean time to recovery

### Dashboards
- GitHub Actions dashboard
- Application metrics (Prometheus/Grafana)
- Security metrics (Datadog)
- Performance metrics (K6 dashboard)

## Maintenance

### Regular Tasks
1. **Weekly**: Review dependency updates (Dependabot)
2. **Monthly**: Update base Docker images
3. **Quarterly**: Review and update security policies
4. **Annually**: Security audit and compliance review

### Updates
- Pipeline configurations are version controlled
- Changes require code review
- Test changes in feature branches first
- Document significant changes

## Support

For questions or issues:
1. Check this documentation
2. Review workflow logs
3. Contact the DevOps team
4. Create an issue in the repository

---

*This guide is maintained by the DevOps team and updated regularly to reflect current practices and configurations.*