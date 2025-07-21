# Incident Response Runbook - Hotel Reviews Microservice

## Overview

This runbook provides step-by-step procedures for responding to production incidents affecting the Hotel Reviews microservice. It covers incident classification, escalation procedures, resolution steps, and post-incident activities.

## Table of Contents

1. [Incident Classification](#incident-classification)
2. [Initial Response](#initial-response)
3. [Escalation Procedures](#escalation-procedures)
4. [Common Incident Types](#common-incident-types)
5. [Resolution Procedures](#resolution-procedures)
6. [Communication Guidelines](#communication-guidelines)
7. [Post-Incident Activities](#post-incident-activities)
8. [Tools and Resources](#tools-and-resources)

## Incident Classification

### Severity Levels

#### P0 - Critical (Service Down)
- **Definition**: Complete service outage affecting all users
- **Response Time**: Immediate (< 5 minutes)
- **Examples**: API completely unreachable, database down, data corruption
- **Escalation**: Immediate to on-call engineer and management

#### P1 - High (Major Degradation)
- **Definition**: Significant service degradation affecting majority of users
- **Response Time**: < 15 minutes
- **Examples**: >50% error rate, response times >5 seconds, core features broken
- **Escalation**: On-call engineer, engineering manager within 30 minutes

#### P2 - Medium (Partial Impact)
- **Definition**: Service degradation affecting subset of users or features
- **Response Time**: < 1 hour
- **Examples**: 10-50% error rate, specific feature broken, performance issues
- **Escalation**: On-call engineer, can wait for business hours for management

#### P3 - Low (Minor Issues)
- **Definition**: Minor issues with minimal user impact
- **Response Time**: < 4 hours (business hours)
- **Examples**: <10% error rate, non-critical feature issues, cosmetic problems
- **Escalation**: Regular engineering team, no immediate management escalation

## Initial Response

### 1. Incident Detection

#### Automated Alerts
```bash
# Check alerting systems
./scripts/monitoring/check-alerts.sh --active --severity high

# Review recent alerts
./scripts/monitoring/alert-history.sh --last 1h
```

#### Manual Detection
```bash
# Quick health check
curl -f https://api.hotel-reviews.com/api/v1/health

# Check key metrics
./scripts/monitoring/check-metrics.sh --environment production --key-metrics
```

### 2. Initial Assessment (First 5 minutes)

```bash
# Set incident variables
export INCIDENT_ID="INC-$(date +%Y%m%d-%H%M%S)"
export INCIDENT_START=$(date --iso-8601)
export RESPONDER=$(whoami)

# Create incident workspace
mkdir -p /tmp/incident-${INCIDENT_ID}
cd /tmp/incident-${INCIDENT_ID}

# Log initial assessment
echo "Incident ID: ${INCIDENT_ID}" > incident.log
echo "Start Time: ${INCIDENT_START}" >> incident.log
echo "Initial Responder: ${RESPONDER}" >> incident.log
```

#### Initial Triage Questions
- [ ] Is the service completely down or partially degraded?
- [ ] How many users are affected?
- [ ] What is the error rate and response time?
- [ ] Are there any recent deployments or changes?
- [ ] Are external dependencies functioning?

### 3. Incident Declaration

```bash
# Declare incident based on severity
./scripts/incident/declare-incident.sh \
  --incident-id ${INCIDENT_ID} \
  --severity P1 \
  --title "API response time degradation" \
  --description "P95 response time increased to 5+ seconds"

# Create incident channel
./scripts/incident/create-incident-channel.sh \
  --incident-id ${INCIDENT_ID} \
  --severity P1
```

## Escalation Procedures

### Escalation Matrix

| Severity | Immediate | 15 min | 30 min | 1 hour |
|----------|-----------|--------|--------|---------|
| P0 | On-call + Manager | Director | VP Eng | CTO |
| P1 | On-call | Manager | Director | VP Eng |
| P2 | On-call | Manager | - | Director |
| P3 | On-call | - | Manager | - |

### Escalation Commands

```bash
# P0 Escalation
./scripts/incident/escalate.sh \
  --incident-id ${INCIDENT_ID} \
  --level P0 \
  --contacts "oncall,manager,director,vp"

# P1 Escalation
./scripts/incident/escalate.sh \
  --incident-id ${INCIDENT_ID} \
  --level P1 \
  --contacts "oncall,manager"
```

## Common Incident Types

### 1. Service Unavailability

#### Symptoms
- HTTP 5xx errors
- Connection timeouts
- Health check failures

#### Investigation Steps
```bash
# Check service status
aws ecs describe-services \
  --cluster hotel-reviews-production-cluster \
  --services hotel-reviews-service

# Check load balancer health
aws elbv2 describe-target-health \
  --target-group-arn ${TARGET_GROUP_ARN}

# Check recent deployments
./scripts/deployment/recent-deployments.sh --limit 5

# Check infrastructure events
aws events describe-events \
  --start-time $(date -d '2 hours ago' --iso-8601) \
  --end-time $(date --iso-8601)
```

#### Resolution Steps
```bash
# If deployment-related, rollback
./scripts/deployment/rollback.sh \
  --environment production \
  --immediate \
  --reason "Service unavailability - incident ${INCIDENT_ID}"

# If infrastructure issue, restart service
aws ecs update-service \
  --cluster hotel-reviews-production-cluster \
  --service hotel-reviews-service \
  --force-new-deployment

# Scale up if capacity issue
aws ecs update-service \
  --cluster hotel-reviews-production-cluster \
  --service hotel-reviews-service \
  --desired-count 5
```

### 2. Database Connection Issues

#### Symptoms
- Database connection errors
- Timeout errors
- Connection pool exhaustion

#### Investigation Steps
```bash
# Check database status
aws rds describe-db-instances \
  --db-instance-identifier hotel-reviews-production-db

# Check connection count
psql -h ${DB_HOST} -U ${DB_USER} -d ${DB_NAME} -c \
  "SELECT count(*) FROM pg_stat_activity WHERE state = 'active';"

# Check slow queries
psql -h ${DB_HOST} -U ${DB_USER} -d ${DB_NAME} -c \
  "SELECT query, now() - query_start AS duration FROM pg_stat_activity WHERE state = 'active' ORDER BY duration DESC;"
```

#### Resolution Steps
```bash
# Kill long-running queries if needed
psql -h ${DB_HOST} -U ${DB_USER} -d ${DB_NAME} -c \
  "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'active' AND now() - query_start > interval '5 minutes';"

# Restart application to reset connection pool
aws ecs update-service \
  --cluster hotel-reviews-production-cluster \
  --service hotel-reviews-service \
  --force-new-deployment

# Scale database if needed
aws rds modify-db-instance \
  --db-instance-identifier hotel-reviews-production-db \
  --db-instance-class db.r5.xlarge \
  --apply-immediately
```

### 3. High Error Rate

#### Symptoms
- Error rate > 5%
- Specific endpoints failing
- Increased 4xx/5xx responses

#### Investigation Steps
```bash
# Check error breakdown by endpoint
aws logs filter-log-events \
  --log-group-name /ecs/hotel-reviews-service/app \
  --start-time $(date -d '30 minutes ago' +%s)000 \
  --filter-pattern '{ $.level = "ERROR" }' \
  --query 'events[*].message'

# Check error patterns
./scripts/monitoring/error-analysis.sh \
  --time-range 30m \
  --group-by endpoint

# Check external dependencies
./scripts/health-checks/dependency-check.sh \
  --environment production \
  --detailed
```

#### Resolution Steps
```bash
# If specific endpoint issue, disable endpoint
./scripts/deployment/toggle-feature.sh \
  --feature broken-endpoint \
  --action disable \
  --environment production

# If dependency issue, enable circuit breaker
./scripts/monitoring/enable-circuit-breaker.sh \
  --service external-api \
  --threshold 50

# If code issue, rollback to previous version
./scripts/deployment/rollback.sh \
  --environment production \
  --to-previous-stable
```

### 4. Performance Degradation

#### Symptoms
- Response times > SLA
- CPU/Memory high
- Queue backlogs

#### Investigation Steps
```bash
# Check resource utilization
./scripts/monitoring/resource-usage.sh \
  --environment production \
  --time-range 1h

# Check slow endpoints
./scripts/monitoring/slow-endpoints.sh \
  --threshold 1000ms \
  --time-range 30m

# Profile application
./scripts/profiling/cpu-profile.sh \
  --environment production \
  --duration 60

# Check queue depths
./scripts/monitoring/queue-depths.sh \
  --environment production
```

#### Resolution Steps
```bash
# Scale horizontally
aws ecs update-service \
  --cluster hotel-reviews-production-cluster \
  --service hotel-reviews-service \
  --desired-count 6

# Enable caching if not enabled
./scripts/deployment/toggle-feature.sh \
  --feature enhanced-caching \
  --action enable \
  --environment production

# Restart services if memory leak suspected
aws ecs update-service \
  --cluster hotel-reviews-production-cluster \
  --service hotel-reviews-service \
  --force-new-deployment
```

## Resolution Procedures

### 1. Immediate Stabilization

#### Primary Actions
```bash
# Rollback to last known good state
./scripts/deployment/rollback.sh \
  --environment production \
  --to-stable \
  --reason "Incident ${INCIDENT_ID}"

# Enable emergency mode (reduced functionality)
./scripts/deployment/emergency-mode.sh \
  --environment production \
  --enable

# Scale up resources
./scripts/deployment/emergency-scale.sh \
  --environment production \
  --scale-factor 2
```

#### Verification
```bash
# Verify stabilization
./scripts/monitoring/verify-stability.sh \
  --environment production \
  --duration 300  # 5 minutes

# Check key metrics
./scripts/monitoring/check-metrics.sh \
  --environment production \
  --metrics "error_rate,response_time,throughput"
```

### 2. Root Cause Analysis

```bash
# Collect logs around incident time
./scripts/incident/collect-logs.sh \
  --incident-id ${INCIDENT_ID} \
  --start-time "${INCIDENT_START}" \
  --duration 2h

# Analyze deployment timeline
./scripts/incident/deployment-timeline.sh \
  --since "${INCIDENT_START}"

# Check external events
./scripts/incident/external-events.sh \
  --time-range 2h
```

### 3. Permanent Fix

```bash
# Apply hotfix if identified
./scripts/deployment/hotfix-deploy.sh \
  --fix-id ${INCIDENT_ID}-hotfix \
  --environment production \
  --verify-first

# Implement workaround if needed
./scripts/deployment/apply-workaround.sh \
  --incident-id ${INCIDENT_ID} \
  --environment production
```

## Communication Guidelines

### 1. Internal Communication

#### Incident Channel
```bash
# Post regular updates (every 15 minutes for P0/P1)
./scripts/incident/status-update.sh \
  --incident-id ${INCIDENT_ID} \
  --status "Investigating database connection issues" \
  --eta "15 minutes"

# Update incident tracking
./scripts/incident/update-tracker.sh \
  --incident-id ${INCIDENT_ID} \
  --field status \
  --value "In Progress"
```

#### Stakeholder Updates
```bash
# Notify stakeholders
./scripts/incident/notify-stakeholders.sh \
  --incident-id ${INCIDENT_ID} \
  --severity P1 \
  --message "API experiencing elevated error rates. Team investigating."
```

### 2. External Communication

#### Customer Communication
```bash
# Update status page
./scripts/incident/update-status-page.sh \
  --incident-id ${INCIDENT_ID} \
  --status "Investigating" \
  --message "We are currently investigating elevated error rates affecting some API requests."

# Send customer notifications (for P0/P1)
./scripts/incident/notify-customers.sh \
  --incident-id ${INCIDENT_ID} \
  --severity P1 \
  --channels "email,slack"
```

### 3. Communication Templates

#### P0 Initial Notification
```
🚨 INCIDENT ALERT - P0
Incident ID: ${INCIDENT_ID}
Service: Hotel Reviews API
Status: INVESTIGATING
Impact: Complete service outage
ETA: Investigating
Lead: ${RESPONDER}
Channel: #incident-${INCIDENT_ID}
```

#### P1 Initial Notification
```
⚠️ INCIDENT ALERT - P1
Incident ID: ${INCIDENT_ID}
Service: Hotel Reviews API
Status: INVESTIGATING
Impact: Service degradation affecting API responses
ETA: 30 minutes
Lead: ${RESPONDER}
Channel: #incident-${INCIDENT_ID}
```

#### Resolution Notification
```
✅ INCIDENT RESOLVED
Incident ID: ${INCIDENT_ID}
Duration: ${DURATION}
Root Cause: ${ROOT_CAUSE}
Resolution: ${RESOLUTION}
Next Steps: Post-incident review scheduled
```

## Post-Incident Activities

### 1. Incident Closure

```bash
# Mark incident as resolved
./scripts/incident/resolve-incident.sh \
  --incident-id ${INCIDENT_ID} \
  --resolution "Rolled back deployment, root cause: database migration bug" \
  --resolved-by ${RESPONDER}

# Update status page
./scripts/incident/update-status-page.sh \
  --incident-id ${INCIDENT_ID} \
  --status "Resolved" \
  --message "All services are now operating normally."
```

### 2. Data Collection

```bash
# Generate incident report
./scripts/incident/generate-report.sh \
  --incident-id ${INCIDENT_ID} \
  --include-timeline \
  --include-logs \
  --include-metrics

# Archive incident data
./scripts/incident/archive-data.sh \
  --incident-id ${INCIDENT_ID} \
  --retention-period 90d
```

### 3. Post-Incident Review (PIR)

#### PIR Preparation
```bash
# Schedule PIR meeting
./scripts/incident/schedule-pir.sh \
  --incident-id ${INCIDENT_ID} \
  --within-days 3

# Collect feedback
./scripts/incident/collect-feedback.sh \
  --incident-id ${INCIDENT_ID} \
  --stakeholders "engineering,product,support"
```

#### PIR Template
```markdown
## Post-Incident Review: ${INCIDENT_ID}

### Incident Summary
- **Date**: ${INCIDENT_DATE}
- **Duration**: ${DURATION}
- **Severity**: P1
- **Impact**: Service degradation affecting 45% of API requests

### Timeline
- **T+0**: Alert triggered for high error rate
- **T+5**: On-call engineer responded, incident declared
- **T+15**: Root cause identified (database migration issue)
- **T+25**: Rollback initiated
- **T+35**: Service restored

### Root Cause
Database migration introduced index conflict causing query timeouts

### What Went Well
- Quick detection via monitoring
- Effective rollback procedure
- Good communication with stakeholders

### What Could Be Improved
- Migration testing process
- Pre-deployment validation
- Monitoring coverage for database queries

### Action Items
1. [ ] Improve migration testing process (Owner: DB Team, Due: 2 weeks)
2. [ ] Add database query monitoring (Owner: SRE Team, Due: 1 week)
3. [ ] Update deployment checklist (Owner: DevOps Team, Due: 3 days)
```

### 4. Follow-up Actions

```bash
# Track action items
./scripts/incident/track-actions.sh \
  --incident-id ${INCIDENT_ID} \
  --import-from-pir

# Update runbooks based on learnings
./scripts/incident/update-runbooks.sh \
  --incident-id ${INCIDENT_ID} \
  --improvements-file pir-improvements.md

# Schedule retrospective
./scripts/incident/schedule-retrospective.sh \
  --incident-id ${INCIDENT_ID} \
  --team engineering
```

## Tools and Resources

### Monitoring and Observability

```bash
# Primary monitoring dashboard
open https://monitoring.hotel-reviews.com/dashboard/production

# Log aggregation
open https://logs.hotel-reviews.com/app/kibana

# APM traces
open https://apm.hotel-reviews.com/app/apm

# Infrastructure monitoring
open https://infra.hotel-reviews.com/dashboard
```

### Communication Tools

- **Incident Channel**: #incident-${INCIDENT_ID}
- **War Room**: Zoom meeting for P0/P1 incidents
- **Status Page**: https://status.hotel-reviews.com
- **PagerDuty**: https://company.pagerduty.com

### Useful Scripts

```bash
# Incident toolkit
./scripts/incident/toolkit.sh --help

# Quick diagnostics
./scripts/diagnostics/quick-check.sh --environment production

# Emergency procedures
./scripts/emergency/procedures.sh --list

# Rollback utilities
./scripts/deployment/rollback-utils.sh --help
```

### Documentation

- **Architecture Diagrams**: `/docs/architecture/`
- **API Documentation**: `/docs/api/`
- **Deployment Procedures**: `/docs/deployment/`
- **Contact Information**: `/docs/contacts.md`

### Contact Information

#### Primary Response Team
- **On-Call Engineer**: See PagerDuty rotation
- **Incident Commander**: Determined by severity
- **Engineering Manager**: manager@company.com
- **SRE Team**: sre@company.com

#### Escalation Contacts
- **Director of Engineering**: director@company.com
- **VP of Engineering**: vp@company.com
- **CTO**: cto@company.com

#### External Support
- **AWS Support**: Enterprise support case
- **Database Support**: DBA team (dba@company.com)
- **Network/CDN Support**: Network team (network@company.com)

---

**Last Updated**: $(date)
**Version**: 1.2
**Owner**: SRE Team