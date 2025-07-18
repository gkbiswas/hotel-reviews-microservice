# Comprehensive Monitoring Implementation

This document describes the comprehensive monitoring implementation for the Hotel Reviews microservice, including Prometheus metrics, distributed tracing with Jaeger, health checks, and custom business metrics.

## Overview

The monitoring system consists of several key components:

- **Prometheus Metrics**: For collecting and storing time-series data
- **Jaeger Distributed Tracing**: For tracking requests across service boundaries
- **Health Checks**: For monitoring service and dependency health
- **Business Metrics**: For tracking domain-specific KPIs
- **SLI/SLO Monitoring**: For service level objectives and indicators
- **Alerting**: For proactive issue detection and notification

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │    │   Prometheus    │    │     Jaeger      │
│                 │───▶│   (Metrics)     │    │   (Tracing)     │
│   Monitoring    │    │                 │    │                 │
│   Middleware    │    └─────────────────┘    └─────────────────┘
│                 │              │                       │
└─────────────────┘              │                       │
         │                       ▼                       ▼
         │                ┌─────────────────┐    ┌─────────────────┐
         │                │    Grafana      │    │  Jaeger UI      │
         │                │  (Dashboard)    │    │ (Trace View)    │
         │                └─────────────────┘    └─────────────────┘
         │
         ▼
┌─────────────────┐
│  Health Checks  │
│   /health       │
│   /healthz      │
│   /readiness    │
│   /liveness     │
└─────────────────┘
```

## Components

### 1. Prometheus Metrics

#### System Metrics
- `hotel_reviews_http_requests_total` - Total HTTP requests
- `hotel_reviews_http_request_duration_seconds` - HTTP request duration
- `hotel_reviews_http_response_size_bytes` - HTTP response size
- `hotel_reviews_http_requests_in_flight` - Current HTTP requests being processed

#### Business Metrics
- `hotel_reviews_processed_total` - Total reviews processed
- `hotel_reviews_rejected_total` - Total reviews rejected
- `hotel_reviews_files_processed_total` - Total files processed
- `hotel_reviews_data_quality_score` - Data quality score by provider

#### Database Metrics
- `hotel_reviews_database_connections` - Database connection pool metrics
- `hotel_reviews_database_queries_total` - Total database queries
- `hotel_reviews_database_query_duration_seconds` - Database query duration
- `hotel_reviews_database_errors_total` - Database errors

#### Cache Metrics
- `hotel_reviews_cache_hits_total` - Cache hits
- `hotel_reviews_cache_misses_total` - Cache misses
- `hotel_reviews_cache_operations_total` - Cache operations

#### S3 Metrics
- `hotel_reviews_s3_operations_total` - S3 operations
- `hotel_reviews_s3_operation_duration_seconds` - S3 operation duration
- `hotel_reviews_s3_errors_total` - S3 errors

### 2. Distributed Tracing

#### Jaeger Integration
- Service name: `hotel-reviews-api`
- Trace sampling: Configurable (default 10%)
- Span tags: HTTP method, endpoint, status code, user agent
- Custom spans for business operations

#### Traced Operations
- HTTP requests
- Database queries
- S3 operations
- File processing
- Review processing
- Cache operations

### 3. Health Checks

#### Endpoints
- `/health` - Overall health status with details
- `/healthz` - Simple health check (OK/Service Unavailable)
- `/readiness` - Readiness probe for Kubernetes
- `/liveness` - Liveness probe for Kubernetes

#### Dependency Health Checks
- **Database**: Connection and query testing
- **Redis**: Ping and basic operations
- **S3**: Bucket access and connectivity
- **External APIs**: HTTP endpoint availability

### 4. Business Metrics

#### Review Processing
- Review ingestion rate
- Review validation success/failure
- Review deduplication hits
- Data quality scores by provider

#### File Processing
- File processing throughput
- File processing errors
- File size distribution
- Records per file

#### Provider Metrics
- Provider response times
- Provider error rates
- Provider data quality
- Provider availability

### 5. SLI/SLO Monitoring

#### Service Level Indicators (SLIs)
- **Availability**: Percentage of successful requests
- **Latency**: 95th and 99th percentile response times
- **Error Rate**: Percentage of failed requests
- **Throughput**: Requests per second

#### Service Level Objectives (SLOs)
- **API Availability**: 99.5% over 30 days
- **API Latency P95**: < 500ms over 1 hour
- **API Latency P99**: < 2 seconds over 1 hour
- **API Error Rate**: < 1% over 1 hour
- **Processing Latency**: < 10 seconds for 95% of reviews

### 6. Alerting Rules

#### Critical Alerts
- Service down
- High error rate (> 5%)
- Database connection failures
- SLO breaches

#### Warning Alerts
- High latency (> 2 seconds)
- Low cache hit rate (< 70%)
- High memory usage
- Processing backlog growth

#### SLO Alerts
- Availability below 99.5%
- Latency above targets
- Error rate above 1%

## Configuration

### Environment Variables

```bash
# Monitoring Configuration
HOTEL_REVIEWS_MONITORING_ENABLED=true
HOTEL_REVIEWS_TRACING_ENABLED=true
HOTEL_REVIEWS_JAEGER_ENDPOINT=http://jaeger:14268/api/traces
HOTEL_REVIEWS_METRICS_ENABLED=true
```

### Docker Compose

The monitoring stack includes:
- Prometheus (metrics collection)
- Grafana (visualization)
- Jaeger (distributed tracing)
- Alertmanager (alert routing)
- Node Exporter (system metrics)
- Postgres Exporter (database metrics)
- Redis Exporter (cache metrics)

## Usage

### Starting the Monitoring Stack

```bash
# Start all services including monitoring
docker-compose --profile monitoring up -d

# Start only the application
docker-compose up -d hotel-reviews-api postgres redis minio
```

### Accessing Monitoring Tools

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)
- **Jaeger UI**: http://localhost:16686
- **Alertmanager**: http://localhost:9093

### Health Check Endpoints

- **Health**: http://localhost:8080/health
- **Metrics**: http://localhost:8080/metrics
- **SLO Report**: http://localhost:8080/slo-report

## Dashboards

### Grafana Dashboards

1. **Application Overview**
   - Request rate, latency, error rate
   - Database performance
   - Cache hit rate
   - System resources

2. **Business Metrics**
   - Review processing rate
   - File processing statistics
   - Provider performance
   - Data quality trends

3. **SLI/SLO Dashboard**
   - SLO compliance
   - SLI trends
   - Error budgets
   - Alert status

### Custom Metrics Queries

```promql
# Request rate
rate(hotel_reviews_http_requests_total[5m])

# Error rate
rate(hotel_reviews_http_requests_total{status_code=~"5.."}[5m]) / rate(hotel_reviews_http_requests_total[5m])

# 95th percentile latency
histogram_quantile(0.95, rate(hotel_reviews_http_request_duration_seconds_bucket[5m]))

# Database connection utilization
hotel_reviews_database_connections{state="in_use"} / hotel_reviews_database_connections{state="open"}
```

## Alerting

### Alert Channels

1. **Email**: For team notifications
2. **Slack**: For real-time alerts
3. **PagerDuty**: For critical incidents
4. **Webhook**: For integration with other tools

### Alert Routing

- **Critical**: Immediate notification to on-call team
- **Warning**: Team notification during business hours
- **SLO**: SRE team for trend analysis

## Best Practices

### Metrics
- Use consistent labeling
- Avoid high cardinality metrics
- Implement proper metric cleanup
- Use histograms for latency measurements

### Tracing
- Add meaningful span attributes
- Use sampling to control overhead
- Implement trace context propagation
- Add custom business spans

### Health Checks
- Keep checks lightweight
- Include dependency status
- Return detailed error information
- Implement circuit breakers

### Alerting
- Define clear runbooks
- Avoid alert fatigue
- Use alert grouping
- Implement alert escalation

## Troubleshooting

### Common Issues

1. **High Memory Usage**
   - Check metric cardinality
   - Implement metric cleanup
   - Adjust retention policies

2. **Missing Traces**
   - Verify Jaeger configuration
   - Check sampling rate
   - Ensure proper context propagation

3. **False Positive Alerts**
   - Review alert thresholds
   - Add alert conditions
   - Implement alert suppression

### Debugging

```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets

# Check Jaeger health
curl http://localhost:16686/api/health

# Check application metrics
curl http://localhost:8080/metrics

# Check health status
curl http://localhost:8080/health
```

## Scaling Considerations

### High Volume Deployments

1. **Metrics**
   - Use Prometheus federation
   - Implement metric relabeling
   - Use recording rules

2. **Tracing**
   - Implement head-based sampling
   - Use trace storage backends
   - Implement trace aggregation

3. **Health Checks**
   - Implement health check caching
   - Use circuit breakers
   - Implement graceful degradation

## Security

### Access Control
- Prometheus: IP restrictions
- Grafana: Authentication required
- Jaeger: Network policies
- Alertmanager: Webhook validation

### Data Protection
- Metric scraping over HTTPS
- Trace data encryption
- Alert webhook authentication
- Sensitive data masking

## Maintenance

### Regular Tasks

1. **Metrics Cleanup**
   - Remove unused metrics
   - Adjust retention policies
   - Monitor storage usage

2. **Alert Tuning**
   - Review alert frequency
   - Update thresholds
   - Add new alerts

3. **Dashboard Updates**
   - Add new metrics
   - Update visualizations
   - Review dashboard usage

### Backup and Recovery

- Prometheus data backup
- Grafana dashboard export
- Alertmanager configuration backup
- Jaeger trace export

## Integration

### CI/CD Pipeline
- Metrics validation
- SLO compliance checks
- Alert rule testing
- Dashboard updates

### External Tools
- APM integration
- Log aggregation
- Incident management
- Change tracking

This comprehensive monitoring implementation provides full observability into the Hotel Reviews microservice, enabling proactive issue detection, performance optimization, and business insight generation.