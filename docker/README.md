# Hotel Reviews Docker Monitoring Stack

This directory contains the complete Docker Compose setup for the Hotel Reviews application with a comprehensive monitoring stack.

## 🚀 Quick Start

```bash
# Start the complete stack with monitoring
./monitoring-stack.sh start

# Or manually with Docker Compose
docker-compose --profile monitoring up -d
```

## 📁 Directory Structure

```
docker/
├── monitoring-stack.sh          # Management script
├── docker-compose.yml           # Main compose file with all services
├── prometheus.yml               # Prometheus configuration
├── alertmanager.yml            # AlertManager configuration
├── nginx.conf                  # Nginx configuration
├── init-db.sql                 # Database initialization
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/
│   │   │   └── prometheus.yml   # Grafana datasource config
│   │   └── dashboards/
│   │       └── dashboard.yml    # Dashboard provisioning config
│   └── dashboards/
│       ├── application-metrics.json      # Application dashboard
│       ├── infrastructure-metrics.json   # Infrastructure dashboard
│       ├── business-metrics.json         # Business metrics dashboard
│       └── slo-sli-dashboard.json       # SLO/SLI dashboard
└── README.md                   # This file
```

## 🐳 Docker Compose Profiles

The setup uses Docker Compose profiles to manage different deployment scenarios:

- **Default profile**: Core application services (API, database, cache, storage)
- **monitoring profile**: Monitoring stack (Prometheus, Grafana, AlertManager, Jaeger, exporters)
- **production profile**: Production services (Nginx reverse proxy, additional exporters)

### Starting Different Profiles

```bash
# Core services only
docker-compose up -d

# Core services + monitoring
docker-compose --profile monitoring up -d

# All services including production components
docker-compose --profile monitoring --profile production up -d
```

## 📊 Services Overview

### Application Services
- **hotel-reviews-api**: Main application (port 8080)
- **postgres**: PostgreSQL database (port 5432)
- **redis**: Redis cache (port 6379)
- **minio**: S3-compatible storage (ports 9000, 9001)

### Monitoring Services
- **prometheus**: Metrics collection (port 9090)
- **grafana**: Visualization dashboard (port 3000)
- **alertmanager**: Alert routing (port 9093)
- **jaeger**: Distributed tracing (port 16686)

### Exporters
- **node-exporter**: System metrics (port 9100)
- **postgres-exporter**: Database metrics (port 9187)
- **redis-exporter**: Cache metrics (port 9121)
- **nginx-exporter**: Web server metrics (port 9113)

### Production Services
- **nginx**: Reverse proxy and load balancer (ports 80, 443)

## 🔧 Configuration Files

### prometheus.yml
Defines metric scraping targets and alert rules:
- Application metrics from hotel-reviews-api
- Infrastructure metrics from various exporters
- Alert rule configuration

### alertmanager.yml
Configures alert routing and notification channels:
- Email notifications for different severity levels
- Slack integration for critical alerts
- PagerDuty integration for SLO breaches

### Grafana Configuration
- **Datasources**: Auto-provisioned Prometheus and Jaeger
- **Dashboards**: Pre-configured dashboards for different metric categories
- **Plugins**: Includes useful visualization plugins

## 📈 Pre-configured Dashboards

1. **Application Metrics**: Service health, request rates, error rates, response times
2. **Infrastructure Metrics**: System resources, database performance, cache metrics
3. **Business Metrics**: Review processing, data quality, provider performance
4. **SLO/SLI Dashboard**: Service level indicators and objectives tracking

## 🚨 Alerting Rules

The stack includes comprehensive alerting rules for:
- **Availability**: Service down, health check failures
- **Performance**: High latency, slow queries
- **Errors**: High error rates, processing failures
- **Resources**: Memory/CPU usage, connection pool exhaustion
- **Business**: Low processing rates, data quality issues

## 🛠️ Management Script

The `monitoring-stack.sh` script provides convenient commands:

```bash
./monitoring-stack.sh start      # Start the stack
./monitoring-stack.sh stop       # Stop the stack
./monitoring-stack.sh restart    # Restart the stack
./monitoring-stack.sh status     # Show service URLs
./monitoring-stack.sh health     # Check service health
./monitoring-stack.sh logs       # Show service logs
./monitoring-stack.sh stats      # Show resource usage
./monitoring-stack.sh backup     # Backup monitoring data
./monitoring-stack.sh update     # Reload configurations
./monitoring-stack.sh load       # Generate test load
```

## 🔍 Accessing Services

After starting the stack, access the services at:

- **Application**: http://localhost:8080
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **AlertManager**: http://localhost:9093
- **Jaeger**: http://localhost:16686
- **MinIO Console**: http://localhost:9001 (minioadmin/minioadmin)

## 📚 Documentation

For detailed documentation, see the main [MONITORING_STACK_GUIDE.md](../MONITORING_STACK_GUIDE.md) file.

## 🐛 Troubleshooting

### Common Issues

1. **Port conflicts**: Ensure ports 3000, 8080, 9090, 9093, 16686 are available
2. **Memory issues**: The stack requires at least 4GB RAM
3. **Permission issues**: Ensure Docker has proper permissions

### Checking Service Health

```bash
# Check all container status
docker-compose ps

# Check specific service logs
docker-compose logs <service-name>

# Check service health endpoints
curl http://localhost:8080/api/v1/health    # Application
curl http://localhost:9090/-/healthy        # Prometheus
curl http://localhost:3000/api/health       # Grafana
```

### Prometheus Targets

Check if all targets are being scraped correctly:
```bash
curl http://localhost:9090/api/v1/targets
```

## 🔄 Updates and Maintenance

### Updating Configurations

1. **Prometheus**: Edit `prometheus.yml` and run `./monitoring-stack.sh update`
2. **AlertManager**: Edit `alertmanager.yml` and restart AlertManager
3. **Grafana**: Dashboards are auto-provisioned from `grafana/dashboards/`

### Data Backup

```bash
# Create backup of monitoring data
./monitoring-stack.sh backup

# Manual backup of specific volumes
docker run --rm -v hotel-reviews_prometheus_data:/data -v $(pwd):/backup alpine tar czf /backup/prometheus-backup.tar.gz -C /data .
```

## 🚀 Production Considerations

For production deployment:

1. **Change default passwords** in environment variables
2. **Configure SSL/TLS** for secure communication
3. **Set up external storage** for persistent data
4. **Configure proper alert channels** with real notification endpoints
5. **Implement backup strategies** for monitoring data
6. **Use secrets management** for sensitive configuration
7. **Consider high availability** setup with multiple instances