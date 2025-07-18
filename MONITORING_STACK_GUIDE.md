# Hotel Reviews Monitoring Stack Guide

This guide provides comprehensive documentation for the Hotel Reviews monitoring stack, which includes Prometheus, Grafana, Jaeger, and AlertManager with pre-configured dashboards and alerting rules.

## 🚀 Quick Start

### Prerequisites

- Docker and Docker Compose installed
- At least 4GB of available RAM
- Ports 3000, 8080, 9090, 9093, 16686 available

### Starting the Monitoring Stack

```bash
# Navigate to the docker directory
cd docker

# Start the complete monitoring stack
./monitoring-stack.sh start

# Or manually with Docker Compose
docker-compose --profile monitoring up -d
```

## 📊 Monitoring Components

### Core Monitoring Services

| Service | URL | Default Credentials | Description |
|---------|-----|-------------------|-------------|
| **Grafana** | http://localhost:3000 | admin/admin | Visualization and dashboards |
| **Prometheus** | http://localhost:9090 | - | Metrics collection and storage |
| **AlertManager** | http://localhost:9093 | - | Alert routing and notifications |
| **Jaeger** | http://localhost:16686 | - | Distributed tracing |

### Application Services

| Service | URL | Description |
|---------|-----|-------------|
| **Hotel Reviews API** | http://localhost:8080 | Main application API |
| **Health Check** | http://localhost:8080/api/v1/health | Application health endpoint |
| **Metrics** | http://localhost:8080/api/v1/metrics | Application metrics endpoint |

### Infrastructure Services

| Service | URL | Default Credentials | Description |
|---------|-----|-------------------|-------------|
| **PostgreSQL** | localhost:5432 | postgres/postgres | Database |
| **Redis** | localhost:6379 | - | Cache |
| **MinIO** | http://localhost:9001 | minioadmin/minioadmin | S3-compatible storage |

### Exporters

| Exporter | URL | Description |
|----------|-----|-------------|
| **Node Exporter** | http://localhost:9100 | System metrics |
| **PostgreSQL Exporter** | http://localhost:9187 | Database metrics |
| **Redis Exporter** | http://localhost:9121 | Cache metrics |
| **Nginx Exporter** | http://localhost:9113 | Web server metrics |

## 📈 Pre-configured Dashboards

### 1. Application Metrics Dashboard
**Access:** Grafana → Application folder → Hotel Reviews - Application Metrics

**Key Metrics:**
- Service status and uptime
- HTTP request rate and response times
- Error rate and status code distribution
- Memory and CPU usage
- Database connection pool metrics

**Panels:**
- Service Status (UP/DOWN indicator)
- HTTP Request Rate (requests per second)
- Error Rate (percentage of 5xx responses)
- Response Time (50th, 95th, 99th percentiles)
- Memory Usage (heap and system memory)
- Database Connections (open vs in-use)

### 2. Infrastructure Metrics Dashboard
**Access:** Grafana → Infrastructure folder → Hotel Reviews - Infrastructure Metrics

**Key Metrics:**
- System CPU, memory, and disk usage
- Network I/O
- Container resource consumption
- Database and cache performance

**Panels:**
- CPU Usage (by instance)
- Memory Usage (system-wide)
- Disk Usage (by mount point)
- Network I/O (receive/transmit)
- PostgreSQL Connections
- Redis Metrics (memory, clients)
- Container CPU Usage

### 3. Business Metrics Dashboard
**Access:** Grafana → Business Metrics folder → Hotel Reviews - Business Metrics

**Key Metrics:**
- Review processing statistics
- Data quality scores
- Cache performance
- Provider metrics

**Panels:**
- Total Reviews Processed
- Review Ingestion Rate
- Processing Backlog
- File Processing Rate (success/failure)
- Data Quality Score by Provider
- Review Rejection Rate
- Cache Hit Rate
- Provider Response Times
- Data Freshness

### 4. SLO/SLI Dashboard
**Access:** Grafana → Business Metrics folder → Hotel Reviews - SLO/SLI Dashboard

**Key Metrics:**
- Service Level Indicators (SLIs)
- Service Level Objectives (SLOs)
- Error budget tracking

**Panels:**
- Availability SLI (target: 99.5%)
- Latency SLI (target: 500ms)
- Error Rate SLI (target: <1%)
- SLO Compliance Status
- Error Budget Remaining
- Request Success Rate

## 🚨 Alerting

### Alert Categories

1. **Critical Alerts**
   - Service Down
   - High Error Rate (>5%)
   - Health Check Failing
   - Database Down
   - File Processing Stalled
   - SLO Availability Breach

2. **Warning Alerts**
   - High Latency (>2s)
   - Database High Error Rate
   - File Processing Backlog
   - Low Cache Hit Rate
   - Circuit Breaker Open
   - High Memory/CPU Usage

3. **Business Alerts**
   - Low Review Ingestion Rate
   - High Review Rejection Rate
   - Data Quality Degraded
   - Provider Down

### Alert Channels

Alerts are configured to be sent via:
- **Email** (critical and warning alerts)
- **Slack** (critical alerts to #alerts-critical, warnings to #alerts-warning)
- **PagerDuty** (SLO breaches)

### Configuration

Alert rules are defined in `/monitoring/alerting_rules.yml` and AlertManager routing is configured in `docker/alertmanager.yml`.

## 🔧 Management Commands

The monitoring stack includes a management script for common operations:

```bash
# Start the monitoring stack
./monitoring-stack.sh start

# Stop the monitoring stack
./monitoring-stack.sh stop

# Restart the monitoring stack
./monitoring-stack.sh restart

# Show service information and URLs
./monitoring-stack.sh status

# Check health of all services
./monitoring-stack.sh health

# Show logs for a specific service
./monitoring-stack.sh logs prometheus

# Show container resource usage
./monitoring-stack.sh stats

# Backup monitoring data
./monitoring-stack.sh backup

# Update and reload configurations
./monitoring-stack.sh update

# Generate test load for monitoring
./monitoring-stack.sh load

# Show help
./monitoring-stack.sh help
```

## 🔍 Distributed Tracing with Jaeger

### Accessing Traces

1. Open Jaeger UI at http://localhost:16686
2. Select "hotel-reviews-api" from the Service dropdown
3. Choose operation and time range
4. Click "Find Traces"

### Key Features

- **Service Map:** Visual representation of service dependencies
- **Trace Timeline:** Detailed view of request flow
- **Error Analysis:** Filter traces by error status
- **Performance Analysis:** Identify slow operations

### Trace Context

Traces include:
- HTTP request details
- Database queries
- Cache operations
- External API calls
- Error information

## 🎯 Key Metrics Reference

### Application Metrics

```promql
# Request rate
rate(hotel_reviews_http_requests_total[5m])

# Error rate
rate(hotel_reviews_http_requests_total{status_code=~"5.."}[5m]) / rate(hotel_reviews_http_requests_total[5m])

# Response time percentiles
histogram_quantile(0.95, rate(hotel_reviews_http_request_duration_seconds_bucket[5m]))

# Memory usage
hotel_reviews_memory_usage_bytes{type="heap"}

# Database connections
hotel_reviews_database_connections{state="open"}
```

### Infrastructure Metrics

```promql
# CPU usage
100 - (avg by (instance) (irate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)

# Memory usage
100 * (1 - ((node_memory_MemAvailable_bytes or node_memory_MemFree_bytes) / node_memory_MemTotal_bytes))

# Disk usage
100 * (1 - (node_filesystem_avail_bytes{fstype!="tmpfs"} / node_filesystem_size_bytes{fstype!="tmpfs"}))
```

### Business Metrics

```promql
# Review processing rate
rate(hotel_reviews_processed_total[5m])

# Data quality score
hotel_reviews_data_quality_score

# Cache hit rate
rate(hotel_reviews_cache_hits_total[5m]) / (rate(hotel_reviews_cache_hits_total[5m]) + rate(hotel_reviews_cache_misses_total[5m]))
```

## 🔧 Customization

### Adding New Dashboards

1. Create dashboard JSON in `docker/grafana/dashboards/`
2. Update `docker/grafana/provisioning/dashboards/dashboard.yml` if needed
3. Restart Grafana: `docker-compose restart grafana`

### Adding New Alerts

1. Add rules to `monitoring/alerting_rules.yml`
2. Update AlertManager config in `docker/alertmanager.yml` if needed
3. Reload Prometheus: `curl -X POST http://localhost:9090/-/reload`

### Adding New Exporters

1. Add exporter service to `docker/docker-compose.yml`
2. Add scrape config to `docker/prometheus.yml`
3. Restart monitoring stack

## 🚨 Troubleshooting

### Common Issues

**Service Won't Start**
```bash
# Check Docker logs
docker-compose logs <service-name>

# Check service health
./monitoring-stack.sh health
```

**Metrics Not Appearing**
```bash
# Verify Prometheus targets
curl http://localhost:9090/api/v1/targets

# Check Prometheus configuration
curl http://localhost:9090/api/v1/status/config
```

**Dashboard Not Loading**
```bash
# Check Grafana logs
docker-compose logs grafana

# Verify datasource configuration
curl -u admin:admin http://localhost:3000/api/datasources
```

**Alerts Not Firing**
```bash
# Check AlertManager status
curl http://localhost:9093/api/v1/status

# Verify alert rules
curl http://localhost:9090/api/v1/rules
```

### Performance Tuning

**Prometheus Storage**
- Default retention: 200h
- Adjust in `docker/docker-compose.yml` under Prometheus command args

**Grafana Performance**
- Increase query timeout for large datasets
- Use query caching for frequently accessed dashboards

**Resource Limits**
- Monitor container resource usage with `./monitoring-stack.sh stats`
- Adjust Docker resource limits as needed

## 📚 Additional Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [Jaeger Documentation](https://www.jaegertracing.io/docs/)
- [AlertManager Documentation](https://prometheus.io/docs/alerting/latest/alertmanager/)

## 🔒 Security Considerations

- Change default Grafana admin password
- Configure SSL/TLS for production
- Set up proper authentication and authorization
- Review and update alert notification channels
- Secure exporters and metrics endpoints
- Implement network policies for container communication

## 🚀 Production Deployment

For production deployment:

1. **Use external databases** for Prometheus and Grafana storage
2. **Configure high availability** with multiple Prometheus instances
3. **Set up proper backup strategies** for metrics and dashboards
4. **Implement proper security** with authentication and encryption
5. **Configure external alert channels** (PagerDuty, Slack, email)
6. **Monitor the monitoring stack** itself with external health checks
7. **Use infrastructure as code** (Terraform, Kubernetes) for reproducible deployments