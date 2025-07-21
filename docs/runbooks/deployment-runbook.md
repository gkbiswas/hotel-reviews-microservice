# Deployment Runbook - Hotel Reviews Microservice

## Overview

This runbook provides step-by-step procedures for deploying the Hotel Reviews microservice to production environments. It covers normal deployments, emergency procedures, rollback scenarios, and troubleshooting.

## Table of Contents

1. [Pre-Deployment Checklist](#pre-deployment-checklist)
2. [Normal Deployment Process](#normal-deployment-process)
3. [Emergency Deployment](#emergency-deployment)
4. [Rollback Procedures](#rollback-procedures)
5. [Health Checks](#health-checks)
6. [Troubleshooting](#troubleshooting)
7. [Post-Deployment Tasks](#post-deployment-tasks)
8. [Contact Information](#contact-information)

## Pre-Deployment Checklist

### Code Readiness ✅
- [ ] All tests pass (unit, integration, security)
- [ ] Code review completed and approved
- [ ] Security scan passed with no critical issues
- [ ] Performance benchmarks within acceptable range
- [ ] Database migrations tested (if applicable)

### Infrastructure Readiness ✅
- [ ] Staging environment successfully deployed and tested
- [ ] Infrastructure capacity checked (CPU, Memory, Storage)
- [ ] Blue-Green environments verified as healthy
- [ ] Database backup completed
- [ ] Monitoring dashboards accessible

### Team Readiness ✅
- [ ] Deployment lead identified
- [ ] On-call engineer available
- [ ] Stakeholders notified of deployment window
- [ ] Rollback plan reviewed and approved

### Documentation ✅
- [ ] Release notes prepared
- [ ] Configuration changes documented
- [ ] Known issues documented
- [ ] Post-deployment validation plan ready

## Normal Deployment Process

### 1. Pre-Deployment Setup (T-30 minutes)

```bash
# Set environment variables
export ENVIRONMENT=production
export AWS_REGION=us-east-1
export DEPLOYMENT_ID=$(date +%Y%m%d_%H%M%S)
export SLACK_CHANNEL="#deployments"

# Verify access and permissions
aws sts get-caller-identity
kubectl cluster-info
```

### 2. Database Migration (T-20 minutes)

```bash
# Create database backup
./scripts/migrations/backup-database.sh \
  --environment production \
  --backup-id "pre-deploy-${DEPLOYMENT_ID}"

# Check migration status
./scripts/migrations/migrate.sh --status --environment production

# Apply pending migrations (if any)
./scripts/migrations/migrate.sh --environment production --dry-run
./scripts/migrations/migrate.sh --environment production
```

### 3. Application Deployment (T-10 minutes)

```bash
# Start deployment to green environment
./scripts/deployment/blue-green-deploy.sh \
  --image ghcr.io/gkbiswas/hotel-reviews:${GITHUB_SHA} \
  --environment production \
  --target-color green \
  --deployment-id ${DEPLOYMENT_ID}

# Monitor deployment progress
./scripts/deployment/monitor-deployment.sh \
  --environment production \
  --deployment-id ${DEPLOYMENT_ID} \
  --timeout 600
```

### 4. Health Verification (T-5 minutes)

```bash
# Run comprehensive health checks
./scripts/health-checks/post-deployment-check.sh \
  --environment production \
  --target-color green \
  --timeout 300

# Verify specific endpoints
curl -f https://api.hotel-reviews.com/api/v1/health
curl -f https://api.hotel-reviews.com/api/v1/metrics
```

### 5. Traffic Switch (T-0 minutes)

```bash
# Gradual traffic switch (recommended)
./scripts/deployment/traffic-switch.sh \
  --environment production \
  --strategy gradual \
  --canary-percentage 10 \
  --target-color green

# Wait 5 minutes and monitor metrics
sleep 300

# Increase to 50%
./scripts/deployment/traffic-switch.sh \
  --environment production \
  --strategy gradual \
  --canary-percentage 50 \
  --target-color green

# Wait 5 minutes and monitor metrics
sleep 300

# Complete switch to 100%
./scripts/deployment/traffic-switch.sh \
  --environment production \
  --strategy immediate \
  --target-color green
```

### 6. Post-Deployment Monitoring (T+10 minutes)

```bash
# Enable enhanced monitoring
./scripts/monitoring/enable-enhanced-monitoring.sh \
  --environment production \
  --duration 60  # Monitor for 60 minutes

# Check key metrics
./scripts/monitoring/check-metrics.sh \
  --environment production \
  --metrics "response_time,error_rate,throughput"
```

## Emergency Deployment

For critical security fixes or production incidents:

### 1. Emergency Override Process

```bash
# Set emergency deployment flag
export EMERGENCY_DEPLOYMENT=true
export EMERGENCY_REASON="Critical security fix for CVE-2024-XXXX"

# Skip non-critical checks (use with caution)
./scripts/deployment/emergency-deploy.sh \
  --image ghcr.io/gkbiswas/hotel-reviews:${GITHUB_SHA} \
  --environment production \
  --reason "${EMERGENCY_REASON}" \
  --skip-gradual-rollout
```

### 2. Stakeholder Notification

```bash
# Send emergency deployment notification
./scripts/notifications/send-emergency-alert.sh \
  --deployment-id ${DEPLOYMENT_ID} \
  --reason "${EMERGENCY_REASON}" \
  --channels "slack,email,sms"
```

## Rollback Procedures

### Automatic Rollback

The system automatically rolls back if:
- Health checks fail for more than 5 minutes
- Error rate exceeds 5% for more than 2 minutes
- Response time exceeds P95 SLA for more than 3 minutes

### Manual Rollback

#### Quick Rollback (Traffic Switch)
```bash
# Immediate traffic switch back to blue environment
./scripts/deployment/traffic-switch.sh \
  --environment production \
  --strategy immediate \
  --target-color blue \
  --reason "Manual rollback due to issues"
```

#### Full Rollback
```bash
# Complete rollback including database
./scripts/deployment/rollback.sh \
  --environment production \
  --target-deployment ${PREVIOUS_DEPLOYMENT_ID} \
  --include-database \
  --reason "Full rollback required"
```

#### Database Rollback
```bash
# Rollback database migrations only
./scripts/migrations/rollback.sh \
  --environment production \
  --steps 2 \
  --backup-restore "pre-deploy-${DEPLOYMENT_ID}"
```

## Health Checks

### Automated Health Checks

```bash
# Application health
curl -f https://api.hotel-reviews.com/api/v1/health

# Database health
./scripts/health-checks/database-check.sh --environment production

# Dependencies health
./scripts/health-checks/dependency-check.sh --environment production

# Load balancer health
./scripts/health-checks/load-balancer-check.sh --environment production
```

### Health Check Endpoints

| Endpoint | Purpose | Expected Response |
|----------|---------|-------------------|
| `/api/v1/health` | Overall application health | 200 OK with status details |
| `/api/v1/health/ready` | Readiness probe | 200 OK when ready to serve |
| `/api/v1/health/live` | Liveness probe | 200 OK when application is live |
| `/api/v1/metrics` | Prometheus metrics | 200 OK with metrics data |

### Key Metrics to Monitor

```bash
# Response time (should be < 500ms P95)
curl -s https://api.hotel-reviews.com/api/v1/metrics | grep response_time_p95

# Error rate (should be < 1%)
curl -s https://api.hotel-reviews.com/api/v1/metrics | grep error_rate

# Throughput (requests per second)
curl -s https://api.hotel-reviews.com/api/v1/metrics | grep requests_per_second

# Database connections
curl -s https://api.hotel-reviews.com/api/v1/metrics | grep db_connections_active
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Deployment Stuck/Hanging

**Symptoms:**
- Deployment process doesn't complete
- Health checks timing out
- Application not responding

**Investigation:**
```bash
# Check ECS service status
aws ecs describe-services \
  --cluster hotel-reviews-production-cluster \
  --services hotel-reviews-service-green

# Check task definition
aws ecs describe-tasks \
  --cluster hotel-reviews-production-cluster \
  --tasks $(aws ecs list-tasks --cluster hotel-reviews-production-cluster --service-name hotel-reviews-service-green --query 'taskArns[0]' --output text)

# Check application logs
aws logs tail /ecs/hotel-reviews-service/app --follow
```

**Resolution:**
```bash
# Force new deployment
aws ecs update-service \
  --cluster hotel-reviews-production-cluster \
  --service hotel-reviews-service-green \
  --force-new-deployment

# If still stuck, rollback
./scripts/deployment/rollback.sh --environment production --immediate
```

#### 2. High Error Rate After Deployment

**Symptoms:**
- Error rate > 5%
- 5xx responses increasing
- Application logs showing errors

**Investigation:**
```bash
# Check application logs for errors
aws logs filter-log-events \
  --log-group-name /ecs/hotel-reviews-service/app \
  --start-time $(date -d '10 minutes ago' +%s)000 \
  --filter-pattern 'ERROR'

# Check database connectivity
./scripts/health-checks/database-check.sh --environment production --verbose

# Check external dependencies
./scripts/health-checks/dependency-check.sh --environment production --detailed
```

**Resolution:**
```bash
# If error rate > 10%, immediate rollback
if [ "$(./scripts/monitoring/get-error-rate.sh)" -gt "10" ]; then
  ./scripts/deployment/rollback.sh --environment production --immediate
fi

# Otherwise, investigate and fix
# Check for configuration issues, network problems, etc.
```

#### 3. Database Migration Failures

**Symptoms:**
- Migration script fails
- Database connectivity issues
- Data corruption concerns

**Investigation:**
```bash
# Check migration status
./scripts/migrations/migrate.sh --status --environment production

# Check database logs
aws rds describe-db-log-files \
  --db-instance-identifier hotel-reviews-production-db

# Verify database connectivity
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT version();"
```

**Resolution:**
```bash
# Restore from backup if necessary
./scripts/migrations/backup-database.sh \
  --restore \
  --backup-file "pre-deploy-${DEPLOYMENT_ID}.sql" \
  --environment production

# Retry migration with verbose logging
./scripts/migrations/migrate.sh \
  --environment production \
  --verbose \
  --retry
```

#### 4. Performance Degradation

**Symptoms:**
- Response times > 1000ms P95
- CPU/Memory usage high
- Timeout errors

**Investigation:**
```bash
# Check resource utilization
aws cloudwatch get-metric-statistics \
  --namespace "AWS/ECS" \
  --metric-name CPUUtilization \
  --dimensions Name=ServiceName,Value=hotel-reviews-service-green \
  --start-time $(date -d '1 hour ago' --iso-8601) \
  --end-time $(date --iso-8601) \
  --period 300 \
  --statistics Average

# Check application metrics
curl -s https://api.hotel-reviews.com/api/v1/metrics | grep -E "(response_time|cpu_usage|memory_usage)"

# Profile application if needed
./scripts/profiling/cpu-profile.sh --environment production --duration 60
```

**Resolution:**
```bash
# Scale up if resource constrained
aws ecs update-service \
  --cluster hotel-reviews-production-cluster \
  --service hotel-reviews-service-green \
  --desired-count 5

# If performance doesn't improve, rollback
./scripts/deployment/rollback.sh --environment production --reason "Performance degradation"
```

## Post-Deployment Tasks

### 1. Verification (T+30 minutes)

```bash
# Run smoke tests
./scripts/testing/smoke-tests.sh --environment production

# Verify monitoring dashboards
./scripts/monitoring/verify-dashboards.sh --environment production

# Check alerting rules
./scripts/monitoring/verify-alerts.sh --environment production
```

### 2. Documentation Updates (T+60 minutes)

```bash
# Update deployment log
echo "${DEPLOYMENT_ID},$(date),${GITHUB_SHA},success" >> deployments.log

# Update configuration documentation
./scripts/docs/update-config-docs.sh --deployment-id ${DEPLOYMENT_ID}

# Create release notes
./scripts/docs/generate-release-notes.sh --version ${GITHUB_SHA}
```

### 3. Team Notification (T+60 minutes)

```bash
# Send success notification
./scripts/notifications/send-deployment-success.sh \
  --deployment-id ${DEPLOYMENT_ID} \
  --version ${GITHUB_SHA} \
  --channels "slack,email"

# Update status page
./scripts/notifications/update-status-page.sh \
  --status "Deployment completed successfully" \
  --version ${GITHUB_SHA}
```

### 4. Cleanup (T+120 minutes)

```bash
# Clean up old blue environment (after verification)
./scripts/deployment/cleanup-old-environment.sh \
  --environment production \
  --color blue \
  --keep-last 2

# Archive deployment artifacts
./scripts/deployment/archive-artifacts.sh \
  --deployment-id ${DEPLOYMENT_ID} \
  --retention-days 30
```

## Contact Information

### Primary Contacts

| Role | Name | Phone | Slack | Email |
|------|------|-------|-------|-------|
| Deployment Lead | DevOps Team | +1-555-0101 | @devops-team | devops@company.com |
| On-Call Engineer | SRE Team | +1-555-0102 | @sre-oncall | sre@company.com |
| Product Owner | Product Team | +1-555-0103 | @product-team | product@company.com |

### Escalation Path

1. **Level 1**: Deployment Lead
2. **Level 2**: SRE Manager
3. **Level 3**: Engineering Director
4. **Level 4**: CTO

### Communication Channels

- **Slack**: #deployments, #incidents, #engineering
- **Email**: engineering-alerts@company.com
- **Phone**: On-call rotation via PagerDuty
- **Status Page**: https://status.company.com

### External Dependencies

| Service | Contact | Documentation |
|---------|---------|---------------|
| AWS Support | Enterprise Support | https://aws.amazon.com/support/ |
| Database Provider | DBA Team | Internal wiki |
| CDN Provider | Network Team | Vendor documentation |

## Appendix

### Required Permissions

```bash
# AWS permissions needed
aws iam get-user  # Verify AWS access
aws ecs describe-clusters  # ECS access
aws rds describe-db-instances  # RDS access
```

### Useful Commands

```bash
# Quick status check
./scripts/deployment/status.sh --environment production

# Emergency contacts
./scripts/notifications/emergency-contacts.sh

# Recent deployments
./scripts/deployment/list-recent.sh --limit 5

# Current version
./scripts/deployment/current-version.sh --environment production
```

### Environment URLs

- **Production**: https://api.hotel-reviews.com
- **Staging**: https://staging-api.hotel-reviews.com
- **Monitoring**: https://monitoring.hotel-reviews.com
- **Logs**: https://logs.hotel-reviews.com

---

**Last Updated**: $(date)
**Version**: 1.0
**Owner**: DevOps Team