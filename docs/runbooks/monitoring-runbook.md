# Monitoring and Alerting Runbook - Hotel Reviews Microservice

## Overview

This runbook provides comprehensive guidance for monitoring, alerting, and observability for the Hotel Reviews microservice. It covers monitoring setup, alert configuration, troubleshooting procedures, and maintenance tasks.

## Table of Contents

1. [Monitoring Architecture](#monitoring-architecture)
2. [Key Metrics and SLIs](#key-metrics-and-slis)
3. [Alert Configuration](#alert-configuration)
4. [Dashboard Management](#dashboard-management)
5. [Log Management](#log-management)
6. [Troubleshooting](#troubleshooting)
7. [Maintenance Procedures](#maintenance-procedures)
8. [Runbook Procedures](#runbook-procedures)

## Monitoring Architecture

### Stack Components

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │───▶│   Prometheus    │───▶│    Grafana      │
│   (Metrics)     │    │   (Collection)  │    │  (Visualization)│
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CloudWatch    │    │   AlertManager  │    │   PagerDuty     │
│   (AWS Metrics) │    │   (Alerting)    │    │  (Escalation)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Access Information

```bash
# Monitoring endpoints
export PROMETHEUS_URL="https://prometheus.hotel-reviews.com"
export GRAFANA_URL="https://grafana.hotel-reviews.com"
export ALERTMANAGER_URL="https://alertmanager.hotel-reviews.com"

# AWS CloudWatch
export AWS_REGION="us-east-1"
export CLOUDWATCH_NAMESPACE="HotelReviews/Production"
```

## Key Metrics and SLIs

### Service Level Indicators (SLIs)

#### 1. Availability SLI
```bash
# Target: 99.9% availability
# Measurement: Successful requests / Total requests
availability = sum(rate(http_requests_total{code!~"5.."}[5m])) / sum(rate(http_requests_total[5m]))
```

#### 2. Latency SLI
```bash
# Target: 95% of requests < 500ms
# Measurement: P95 response time
latency_p95 = histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))
```

#### 3. Error Rate SLI
```bash
# Target: < 1% error rate
# Measurement: 5xx errors / Total requests
error_rate = sum(rate(http_requests_total{code=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))
```

### Core Metrics

#### Application Metrics
```bash
# Request metrics
http_requests_total{method, endpoint, code}
http_request_duration_seconds{method, endpoint}
http_requests_in_flight

# Business metrics
hotel_reviews_created_total
hotel_reviews_processed_total
active_users_total
search_requests_total

# Resource metrics
go_memstats_alloc_bytes
go_memstats_sys_bytes
go_goroutines
```

#### Infrastructure Metrics
```bash
# Container metrics
container_cpu_usage_seconds_total
container_memory_usage_bytes
container_network_receive_bytes_total
container_network_transmit_bytes_total

# Database metrics
db_connections_active
db_connections_idle
db_query_duration_seconds
db_queries_total
```

### Checking Current Metrics

```bash
# Query Prometheus directly
curl -s "${PROMETHEUS_URL}/api/v1/query?query=up" | jq '.data.result'

# Check application health metrics
curl -s "https://api.hotel-reviews.com/api/v1/metrics" | grep -E "(http_requests|response_time|error_rate)"

# AWS CloudWatch metrics
aws cloudwatch get-metric-statistics \
  --namespace "${CLOUDWATCH_NAMESPACE}" \
  --metric-name "RequestCount" \
  --start-time $(date -d '1 hour ago' --iso-8601) \
  --end-time $(date --iso-8601) \
  --period 300 \
  --statistics Sum
```

## Alert Configuration

### Critical Alerts (P0)

#### Service Down Alert
```yaml
groups:
- name: service_down
  rules:
  - alert: ServiceDown
    expr: up{job="hotel-reviews"} == 0
    for: 1m
    labels:
      severity: critical
      team: sre
    annotations:
      summary: "Hotel Reviews service is down"
      description: "Service {{ $labels.instance }} has been down for more than 1 minute"
      runbook_url: "https://runbooks.hotel-reviews.com/service-down"
```

#### High Error Rate Alert
```yaml
- alert: HighErrorRate
  expr: rate(http_requests_total{code=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
  for: 2m
  labels:
    severity: critical
    team: sre
  annotations:
    summary: "High error rate detected"
    description: "Error rate is {{ $value | humanizePercentage }} for the last 5 minutes"
    runbook_url: "https://runbooks.hotel-reviews.com/high-error-rate"
```

### Warning Alerts (P1)

#### High Latency Alert
```yaml
- alert: HighLatency
  expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1.0
  for: 5m
  labels:
    severity: warning
    team: sre
  annotations:
    summary: "High latency detected"
    description: "P95 latency is {{ $value }}s for the last 5 minutes"
    runbook_url: "https://runbooks.hotel-reviews.com/high-latency"
```

#### Database Connection Issues
```yaml
- alert: DatabaseConnectionHigh
  expr: db_connections_active / db_connections_max > 0.8
  for: 5m
  labels:
    severity: warning
    team: dba
  annotations:
    summary: "Database connection usage high"
    description: "Database connections are at {{ $value | humanizePercentage }} capacity"
    runbook_url: "https://runbooks.hotel-reviews.com/database-connections"
```

### Alert Management Commands

```bash
# List active alerts
curl -s "${ALERTMANAGER_URL}/api/v1/alerts" | jq '.data[] | select(.status.state=="active")'

# Silence alert temporarily
./scripts/monitoring/silence-alert.sh \
  --alert "HighLatency" \
  --duration "2h" \
  --reason "Planned maintenance" \
  --creator "$(whoami)"

# Check alert routing
./scripts/monitoring/test-alert-routing.sh \
  --alert-name "ServiceDown" \
  --dry-run

# Validate alert rules
./scripts/monitoring/validate-alerts.sh \
  --config-file monitoring/alerts/production.yml
```

## Dashboard Management

### Key Dashboards

#### 1. Service Overview Dashboard
```bash
# Access dashboard
open "${GRAFANA_URL}/d/service-overview/hotel-reviews-service-overview"

# Key panels:
# - Request rate (QPS)
# - Error rate percentage
# - Response time percentiles (P50, P95, P99)
# - Service availability
# - Active instances
```

#### 2. Infrastructure Dashboard
```bash
# Access dashboard
open "${GRAFANA_URL}/d/infrastructure/hotel-reviews-infrastructure"

# Key panels:
# - CPU utilization per container
# - Memory usage and limits
# - Network I/O
# - Disk usage
# - Load balancer metrics
```

#### 3. Business Metrics Dashboard
```bash
# Access dashboard
open "${GRAFANA_URL}/d/business/hotel-reviews-business-metrics"

# Key panels:
# - Reviews created per hour
# - Search requests
# - User activity
# - API endpoint usage
# - Geographic distribution
```

#### 4. Database Dashboard
```bash
# Access dashboard
open "${GRAFANA_URL}/d/database/hotel-reviews-database"

# Key panels:
# - Connection pool status
# - Query performance
# - Slow queries
# - Database size and growth
# - Replication lag
```

### Dashboard Maintenance

```bash
# Export dashboard for backup
./scripts/monitoring/export-dashboard.sh \
  --dashboard-id "service-overview" \
  --output-file "dashboards/service-overview-backup.json"

# Import dashboard from JSON
./scripts/monitoring/import-dashboard.sh \
  --input-file "dashboards/new-dashboard.json" \
  --folder "Hotel Reviews"

# Update dashboard variables
./scripts/monitoring/update-dashboard-vars.sh \
  --dashboard-id "service-overview" \
  --environment "production"

# Validate dashboard queries
./scripts/monitoring/validate-dashboard.sh \
  --dashboard-id "service-overview" \
  --time-range "1h"
```

## Log Management

### Log Sources and Locations

#### Application Logs
```bash
# ECS/CloudWatch logs
aws logs describe-log-groups --log-group-name-prefix "/ecs/hotel-reviews"

# Stream application logs
aws logs tail "/ecs/hotel-reviews-service/app" --follow

# Filter logs by level
aws logs filter-log-events \
  --log-group-name "/ecs/hotel-reviews-service/app" \
  --filter-pattern '{ $.level = "ERROR" }' \
  --start-time $(date -d '1 hour ago' +%s)000
```

#### Access Logs
```bash
# Load balancer access logs
aws s3 ls s3://hotel-reviews-alb-logs/production/

# Download recent access logs
aws s3 sync s3://hotel-reviews-alb-logs/production/$(date +%Y/%m/%d)/ ./logs/

# Analyze access patterns
./scripts/monitoring/analyze-access-logs.sh \
  --date $(date +%Y-%m-%d) \
  --top-endpoints 10
```

#### Database Logs
```bash
# RDS logs
aws rds describe-db-log-files \
  --db-instance-identifier hotel-reviews-production-db

# Download database logs
aws rds download-db-log-file-portion \
  --db-instance-identifier hotel-reviews-production-db \
  --log-file-name postgresql.log.2024-01-15-12 \
  --output text > db-logs.txt
```

### Log Analysis Commands

```bash
# Search for errors in application logs
./scripts/monitoring/search-logs.sh \
  --log-group "/ecs/hotel-reviews-service/app" \
  --pattern "ERROR" \
  --time-range "1h" \
  --limit 100

# Generate error summary
./scripts/monitoring/error-summary.sh \
  --time-range "24h" \
  --group-by "error_type,endpoint"

# Analyze request patterns
./scripts/monitoring/request-analysis.sh \
  --time-range "1h" \
  --metrics "rate,errors,latency" \
  --group-by endpoint

# Check for specific user issues
./scripts/monitoring/user-trace.sh \
  --user-id "user123" \
  --time-range "1h" \
  --include-errors
```

### Log Retention and Management

```bash
# Configure log retention
aws logs put-retention-policy \
  --log-group-name "/ecs/hotel-reviews-service/app" \
  --retention-in-days 30

# Archive old logs
./scripts/monitoring/archive-logs.sh \
  --log-group "/ecs/hotel-reviews-service/app" \
  --older-than "30d" \
  --archive-to "s3://hotel-reviews-log-archive"

# Clean up archived logs
./scripts/monitoring/cleanup-archived-logs.sh \
  --older-than "1y" \
  --dry-run
```

## Troubleshooting

### Common Monitoring Issues

#### 1. Missing Metrics

**Problem**: Metrics not appearing in Prometheus/Grafana

**Investigation**:
```bash
# Check if application is exposing metrics
curl -s "https://api.hotel-reviews.com/api/v1/metrics" | head -20

# Check Prometheus targets
curl -s "${PROMETHEUS_URL}/api/v1/targets" | jq '.data.activeTargets[] | select(.labels.job=="hotel-reviews")'

# Check scrape config
kubectl get configmap prometheus-config -o yaml | grep -A 20 "job_name.*hotel-reviews"
```

**Resolution**:
```bash
# Restart Prometheus if needed
kubectl rollout restart deployment/prometheus

# Check network connectivity
kubectl exec -it prometheus-pod -- wget -qO- http://hotel-reviews-service:8080/metrics

# Verify service discovery
./scripts/monitoring/verify-service-discovery.sh --service hotel-reviews
```

#### 2. Alert Not Firing

**Problem**: Expected alerts not triggering

**Investigation**:
```bash
# Check alert rule syntax
./scripts/monitoring/validate-alert-rules.sh \
  --rules-file monitoring/alerts/production.yml

# Test alert query manually
curl -s "${PROMETHEUS_URL}/api/v1/query?query=up{job=\"hotel-reviews\"}" | jq '.data.result'

# Check AlertManager configuration
curl -s "${ALERTMANAGER_URL}/api/v1/status" | jq '.data.configYAML'
```

**Resolution**:
```bash
# Reload alert rules
curl -X POST "${PROMETHEUS_URL}/-/reload"

# Test alert manually
./scripts/monitoring/test-alert.sh \
  --alert-name "ServiceDown" \
  --force-trigger

# Check alert routing
./scripts/monitoring/trace-alert-routing.sh \
  --alert-name "ServiceDown"
```

#### 3. Dashboard Not Loading

**Problem**: Grafana dashboard displays errors or no data

**Investigation**:
```bash
# Check Grafana logs
kubectl logs deployment/grafana | grep ERROR

# Test data source connectivity
curl -s "${GRAFANA_URL}/api/datasources/proxy/1/api/v1/query?query=up" \
  -H "Authorization: Bearer ${GRAFANA_TOKEN}"

# Validate dashboard JSON
./scripts/monitoring/validate-dashboard-json.sh \
  --dashboard-file "dashboards/service-overview.json"
```

**Resolution**:
```bash
# Refresh data source
curl -X POST "${GRAFANA_URL}/api/datasources/1/health" \
  -H "Authorization: Bearer ${GRAFANA_TOKEN}"

# Reimport dashboard
./scripts/monitoring/reimport-dashboard.sh \
  --dashboard-id "service-overview" \
  --force

# Clear Grafana cache
kubectl exec -it grafana-pod -- rm -rf /var/lib/grafana/cache/*
```

### Performance Troubleshooting

#### High Query Load on Prometheus
```bash
# Check query load
curl -s "${PROMETHEUS_URL}/api/v1/label/__name__/values" | jq '. | length'

# Identify expensive queries
./scripts/monitoring/top-queries.sh \
  --prometheus-url "${PROMETHEUS_URL}" \
  --top 10

# Optimize recording rules
./scripts/monitoring/optimize-recording-rules.sh \
  --input-file monitoring/rules/production.yml
```

#### Dashboard Performance Issues
```bash
# Analyze dashboard query performance
./scripts/monitoring/dashboard-query-analysis.sh \
  --dashboard-id "service-overview" \
  --time-range "1h"

# Optimize panel queries
./scripts/monitoring/optimize-panel-queries.sh \
  --dashboard-id "service-overview" \
  --threshold "5s"
```

## Maintenance Procedures

### Daily Maintenance

```bash
# Check monitoring stack health
./scripts/monitoring/health-check.sh --all-components

# Verify alert delivery
./scripts/monitoring/test-alert-delivery.sh --test-mode

# Review overnight alerts
./scripts/monitoring/alert-summary.sh --since "24h"

# Check disk usage
./scripts/monitoring/check-disk-usage.sh --prometheus --grafana
```

### Weekly Maintenance

```bash
# Update monitoring dashboards
./scripts/monitoring/update-dashboards.sh --from-templates

# Review and tune alert thresholds
./scripts/monitoring/alert-threshold-review.sh --suggest-changes

# Clean up old metrics
./scripts/monitoring/cleanup-old-metrics.sh --older-than "90d"

# Backup monitoring configuration
./scripts/monitoring/backup-config.sh --output-dir "./backups/$(date +%Y%m%d)"
```

### Monthly Maintenance

```bash
# Review monitoring costs
./scripts/monitoring/cost-analysis.sh --period "last-month"

# Update monitoring stack
./scripts/monitoring/update-stack.sh --version "latest-stable"

# Capacity planning review
./scripts/monitoring/capacity-planning.sh --forecast-days 90

# Documentation updates
./scripts/monitoring/update-documentation.sh --auto-generate
```

## Runbook Procedures

### Procedure: Add New Alert

```bash
# 1. Create alert rule
cat > monitoring/alerts/new-alert.yml << 'EOF'
groups:
- name: new_alert
  rules:
  - alert: NewAlert
    expr: metric_name > threshold
    for: 5m
    labels:
      severity: warning
      team: engineering
    annotations:
      summary: "New alert description"
      description: "Detailed description with context"
      runbook_url: "https://runbooks.hotel-reviews.com/new-alert"
EOF

# 2. Validate alert rule
./scripts/monitoring/validate-alert-rules.sh \
  --rules-file monitoring/alerts/new-alert.yml

# 3. Test alert query
curl -s "${PROMETHEUS_URL}/api/v1/query?query=metric_name" | jq '.data.result'

# 4. Deploy alert rule
./scripts/monitoring/deploy-alert-rules.sh \
  --rules-file monitoring/alerts/new-alert.yml \
  --environment production

# 5. Test alert firing
./scripts/monitoring/test-alert.sh \
  --alert-name "NewAlert" \
  --simulate
```

### Procedure: Create New Dashboard

```bash
# 1. Create dashboard template
./scripts/monitoring/create-dashboard-template.sh \
  --name "New Dashboard" \
  --service "hotel-reviews" \
  --template "service-dashboard"

# 2. Customize dashboard
# Edit the generated JSON file with your panels

# 3. Validate dashboard
./scripts/monitoring/validate-dashboard.sh \
  --dashboard-file "dashboards/new-dashboard.json"

# 4. Import dashboard
./scripts/monitoring/import-dashboard.sh \
  --input-file "dashboards/new-dashboard.json" \
  --folder "Hotel Reviews"

# 5. Test dashboard
open "${GRAFANA_URL}/d/new-dashboard/new-dashboard"
```

### Procedure: Investigation Workflow

```bash
# 1. Initial triage
./scripts/monitoring/triage.sh \
  --service hotel-reviews \
  --time-range "1h"

# 2. Collect diagnostic information
./scripts/monitoring/collect-diagnostics.sh \
  --service hotel-reviews \
  --output-dir "/tmp/diagnostics-$(date +%Y%m%d-%H%M%S)"

# 3. Analyze metrics and logs
./scripts/monitoring/analyze-issue.sh \
  --time-range "1h" \
  --correlation-analysis \
  --include-logs

# 4. Generate investigation report
./scripts/monitoring/generate-investigation-report.sh \
  --service hotel-reviews \
  --issue-start-time "${ISSUE_START}" \
  --output-file "investigation-report.md"
```

### Useful Monitoring Commands

```bash
# Quick service health check
./scripts/monitoring/quick-health.sh --service hotel-reviews

# Get current SLI values
./scripts/monitoring/current-sli.sh --all

# Check alert status
./scripts/monitoring/alert-status.sh --active-only

# Generate monitoring report
./scripts/monitoring/generate-report.sh --period "last-week"

# Emergency monitoring disable (use with caution)
./scripts/monitoring/emergency-disable.sh --alert "AlertName" --duration "1h"
```

### Documentation and Resources

- **Grafana Documentation**: https://grafana.hotel-reviews.com/docs
- **Prometheus Queries**: `/docs/monitoring/prometheus-queries.md`
- **Alert Runbooks**: `/docs/runbooks/alerts/`
- **Dashboard Templates**: `/monitoring/dashboards/templates/`

---

**Last Updated**: $(date)
**Version**: 1.1
**Owner**: SRE Team