# Hotel Reviews Kubernetes Manifests

This directory contains comprehensive Kubernetes manifests for deploying the Hotel Reviews microservice application with full production-grade features including monitoring, security, backup, and multi-environment support.

## Architecture Overview

The application consists of the following components:

### Core Services
- **API Server**: Main application serving REST API endpoints
- **Worker Service**: Background processing for file ingestion and analytics
- **PostgreSQL**: Primary database with replication support
- **Redis**: Caching layer and session storage
- **Kafka**: Message broker for event-driven architecture
- **ZooKeeper**: Coordination service for Kafka

### Monitoring & Observability
- **Prometheus**: Metrics collection and alerting
- **Grafana**: Visualization and dashboards
- **ServiceMonitor**: Prometheus service discovery
- **PrometheusRule**: Alerting rules with SLA monitoring

### Security & Compliance
- **NetworkPolicy**: Pod-to-pod communication security
- **PodSecurityPolicy**: Pod security standards
- **RBAC**: Role-based access control
- **Secrets Management**: Encrypted credential storage
- **Security Scanning**: Automated vulnerability scanning

### Backup & Disaster Recovery
- **Automated Backups**: Daily database backups to S3
- **Backup Validation**: Monthly disaster recovery tests
- **Cleanup Jobs**: Automated retention management

### High Availability & Scaling
- **HorizontalPodAutoscaler**: CPU/memory-based scaling
- **PodDisruptionBudget**: Maintain availability during updates
- **StatefulSet**: Stateful services with persistent storage
- **Multi-AZ Deployment**: Cross-zone redundancy

## Directory Structure

```
k8s/
├── base/                           # Base Kubernetes manifests
│   ├── configmap.yaml             # Application configuration
│   ├── secret.yaml                # Secrets and credentials
│   ├── rbac.yaml                  # RBAC configuration
│   ├── deployment.yaml            # API deployment
│   ├── worker-deployment.yaml     # Worker deployment
│   ├── service.yaml               # Service definitions
│   ├── supporting-services.yaml   # Supporting service definitions
│   ├── ingress.yaml               # Ingress configuration
│   ├── statefulset.yaml           # StatefulSets for databases
│   ├── pvc.yaml                   # PersistentVolumeClaims
│   ├── hpa.yaml                   # HorizontalPodAutoscaler
│   ├── pdb.yaml                   # PodDisruptionBudget
│   ├── networkpolicy.yaml         # Network security policies
│   ├── jobs.yaml                  # Jobs and CronJobs
│   ├── servicemonitor.yaml        # Prometheus monitoring
│   ├── kustomization.yaml         # Base kustomization
│   └── namespace.yaml             # Namespace definition
├── overlays/
│   ├── staging/                   # Staging environment
│   │   ├── kustomization.yaml
│   │   ├── staging-namespace.yaml
│   │   └── staging-monitoring.yaml
│   └── production/                # Production environment
│       ├── kustomization.yaml
│       ├── production-namespace.yaml
│       ├── production-monitoring.yaml
│       ├── production-backup.yaml
│       └── production-security.yaml
└── README.md                      # This file
```

## Prerequisites

1. **Kubernetes Cluster**: v1.25+
2. **kubectl**: Configured with cluster access
3. **Kustomize**: v4.5+ (or kubectl with kustomize support)
4. **Helm** (optional): For installing operators
5. **Storage Class**: For persistent volumes
6. **Ingress Controller**: NGINX Ingress Controller
7. **Cert-Manager**: For TLS certificate management
8. **Prometheus Operator**: For monitoring stack

### Required Operators/Controllers

```bash
# Install NGINX Ingress Controller
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm install ingress-nginx ingress-nginx/ingress-nginx -n ingress-nginx --create-namespace

# Install Cert-Manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Install Prometheus Operator
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring --create-namespace
```

## Deployment Guide

### 1. Prepare Secrets

Create the required secret files for production:

```bash
# Create secrets directory
mkdir -p k8s/overlays/production/secrets

# Database credentials
echo "your_secure_database_password" > k8s/overlays/production/secrets/database-password
echo "your_jwt_secret_key_that_should_be_at_least_32_characters_long" > k8s/overlays/production/secrets/jwt-secret

# AWS credentials
echo "your_aws_access_key_id" > k8s/overlays/production/secrets/s3-access-key-id
echo "your_aws_secret_access_key" > k8s/overlays/production/secrets/s3-secret-access-key

# Redis password
echo "your_redis_password" > k8s/overlays/production/secrets/redis-password

# Kafka SASL credentials (if using SASL)
echo "your_kafka_username" > k8s/overlays/production/secrets/kafka-sasl-username
echo "your_kafka_password" > k8s/overlays/production/secrets/kafka-sasl-password

# Notification credentials
echo "your_slack_webhook_url" > k8s/overlays/production/secrets/slack-webhook-url
echo "your_email_password" > k8s/overlays/production/secrets/email-password

# TLS certificates
cat your_tls_cert.pem > k8s/overlays/production/secrets/tls-cert
cat your_tls_key.pem > k8s/overlays/production/secrets/tls-key

# Monitoring credentials
echo "your_monitoring_api_key" > k8s/overlays/production/secrets/monitoring-key
```

### 2. Deploy to Staging

```bash
# Validate manifests
kubectl kustomize k8s/overlays/staging --dry-run=client

# Deploy to staging
kubectl apply -k k8s/overlays/staging

# Wait for deployment to complete
kubectl rollout status deployment/hotel-reviews-api -n hotel-reviews-staging
kubectl rollout status deployment/hotel-reviews-worker -n hotel-reviews-staging
kubectl rollout status statefulset/hotel-reviews-postgres -n hotel-reviews-staging
kubectl rollout status statefulset/hotel-reviews-redis -n hotel-reviews-staging
kubectl rollout status statefulset/hotel-reviews-kafka -n hotel-reviews-staging

# Verify deployment
kubectl get pods -n hotel-reviews-staging
kubectl get services -n hotel-reviews-staging
kubectl get ingress -n hotel-reviews-staging
```

### 3. Deploy to Production

```bash
# Validate manifests
kubectl kustomize k8s/overlays/production --dry-run=client

# Deploy to production
kubectl apply -k k8s/overlays/production

# Wait for deployment to complete
kubectl rollout status deployment/hotel-reviews-api -n hotel-reviews-production
kubectl rollout status deployment/hotel-reviews-worker -n hotel-reviews-production
kubectl rollout status statefulset/hotel-reviews-postgres -n hotel-reviews-production
kubectl rollout status statefulset/hotel-reviews-redis -n hotel-reviews-production
kubectl rollout status statefulset/hotel-reviews-kafka -n hotel-reviews-production

# Verify deployment
kubectl get pods -n hotel-reviews-production
kubectl get services -n hotel-reviews-production
kubectl get ingress -n hotel-reviews-production
```

### 4. Verify Deployment

```bash
# Check all resources
kubectl get all -n hotel-reviews-production

# Check persistent volumes
kubectl get pvc -n hotel-reviews-production
kubectl get pv

# Check network policies
kubectl get networkpolicy -n hotel-reviews-production

# Check HPA status
kubectl get hpa -n hotel-reviews-production

# Check PDB status
kubectl get pdb -n hotel-reviews-production

# Check service monitors
kubectl get servicemonitor -n hotel-reviews-production

# Check prometheus rules
kubectl get prometheusrule -n hotel-reviews-production
```

## Configuration

### Environment Variables

The application uses environment variables for configuration. Key variables include:

- `ENVIRONMENT`: Environment name (staging/production)
- `LOG_LEVEL`: Logging level (debug/info/warn/error)
- `DATABASE_HOST`: PostgreSQL host
- `REDIS_HOST`: Redis host
- `KAFKA_BROKERS`: Kafka broker addresses
- `S3_BUCKET`: S3 bucket for file storage
- `JWT_SECRET`: JWT signing secret

### Resource Limits

#### Staging Environment
- **API**: 300m CPU, 256Mi Memory
- **Worker**: 500m CPU, 512Mi Memory
- **PostgreSQL**: 500m CPU, 512Mi Memory
- **Redis**: 200m CPU, 256Mi Memory

#### Production Environment
- **API**: 1000m CPU, 1Gi Memory
- **Worker**: 2000m CPU, 2Gi Memory
- **PostgreSQL**: 2000m CPU, 4Gi Memory
- **Redis**: 1000m CPU, 2Gi Memory

### Storage Requirements

#### Staging Environment
- **PostgreSQL**: 10Gi
- **Redis**: 2Gi
- **Kafka**: 5Gi

#### Production Environment
- **PostgreSQL**: 100Gi
- **Redis**: 20Gi
- **Kafka**: 50Gi

## Monitoring

### Prometheus Metrics

The application exposes metrics on port 9090:

- `http_requests_total`: Total HTTP requests
- `http_request_duration_seconds`: Request duration
- `database_connections_active`: Active database connections
- `cache_hits_total`: Cache hits
- `kafka_consumer_lag`: Kafka consumer lag
- `processing_jobs_total`: Background jobs processed

### Grafana Dashboards

Access Grafana at:
- **Staging**: `https://grafana.staging.hotel-reviews.com`
- **Production**: `https://grafana.hotel-reviews.com`

### Alerting

Prometheus alerts are configured for:
- API downtime (1 minute threshold)
- High error rate (>5% for production, >20% for staging)
- High latency (>2 seconds for production, >5 seconds for staging)
- Database connection issues
- High resource usage
- SLA breaches

## Security

### Network Security

- **Default Deny**: All traffic denied by default
- **Ingress Only**: External traffic only through ingress
- **Internal Communication**: Restricted to required services
- **DNS Access**: Limited to kube-dns

### Pod Security

- **Non-Root**: All containers run as non-root user
- **Read-Only Root**: Root filesystem is read-only
- **No Privileged**: No privileged containers
- **Dropped Capabilities**: All capabilities dropped

### Secrets Management

- **Encrypted at Rest**: All secrets encrypted
- **RBAC Protected**: Access controlled by RBAC
- **Rotation**: Regular secret rotation recommended
- **External Secrets**: Consider using external secret management

## Backup and Disaster Recovery

### Database Backups

- **Frequency**: Daily at 2 AM UTC
- **Retention**: 30 days for production, 7 days for staging
- **Storage**: AWS S3 with cross-region replication
- **Validation**: Monthly disaster recovery tests

### Backup Verification

```bash
# Check backup jobs
kubectl get cronjob -n hotel-reviews-production

# Check backup job logs
kubectl logs -n hotel-reviews-production job/hotel-reviews-production-backup-database-$(date +%Y%m%d)

# List backups in S3
aws s3 ls s3://your-bucket/backups/database/production/
```

### Disaster Recovery

1. **Automated Testing**: Monthly DR tests verify backup integrity
2. **Recovery Procedures**: Documented runbooks for data recovery
3. **RTO/RPO**: Recovery Time Objective: 1 hour, Recovery Point Objective: 24 hours

## Scaling

### Horizontal Pod Autoscaler

- **API**: 3-20 replicas (staging: 2-5)
- **Worker**: 2-10 replicas (staging: 1-3)
- **Metrics**: CPU (70%), Memory (80%), Custom metrics

### Vertical Scaling

For databases and stateful services:
1. Update resource limits in kustomization
2. Apply changes with rolling update
3. Monitor performance metrics

## Troubleshooting

### Common Issues

1. **Pods Not Starting**
   ```bash
   kubectl describe pod <pod-name> -n hotel-reviews-production
   kubectl logs <pod-name> -n hotel-reviews-production
   ```

2. **Database Connection Issues**
   ```bash
   kubectl exec -it hotel-reviews-postgres-0 -n hotel-reviews-production -- psql -U postgres
   kubectl logs hotel-reviews-postgres-0 -n hotel-reviews-production
   ```

3. **Ingress Not Working**
   ```bash
   kubectl describe ingress hotel-reviews-ingress -n hotel-reviews-production
   kubectl logs -n ingress-nginx deployment/ingress-nginx-controller
   ```

4. **HPA Not Scaling**
   ```bash
   kubectl describe hpa hotel-reviews-api-hpa -n hotel-reviews-production
   kubectl top pods -n hotel-reviews-production
   ```

### Debugging Commands

```bash
# Check cluster events
kubectl get events -n hotel-reviews-production --sort-by=.metadata.creationTimestamp

# Check resource usage
kubectl top pods -n hotel-reviews-production
kubectl top nodes

# Check network connectivity
kubectl exec -it <pod-name> -n hotel-reviews-production -- nslookup hotel-reviews-postgres
kubectl exec -it <pod-name> -n hotel-reviews-production -- telnet hotel-reviews-redis 6379

# Check certificate status
kubectl describe certificate hotel-reviews-tls -n hotel-reviews-production
```

## Maintenance

### Regular Tasks

1. **Monthly**:
   - Review and update resource limits
   - Validate backup integrity
   - Security scan results review
   - Certificate renewal check

2. **Quarterly**:
   - Dependency updates
   - Security policy review
   - Disaster recovery testing
   - Performance optimization

3. **Annually**:
   - Architecture review
   - Compliance audit
   - Cost optimization
   - Technology stack updates

### Updates and Rollbacks

```bash
# Update application version
kubectl set image deployment/hotel-reviews-api hotel-reviews-api=hotel-reviews-api:v1.1.0 -n hotel-reviews-production

# Check rollout status
kubectl rollout status deployment/hotel-reviews-api -n hotel-reviews-production

# Rollback if needed
kubectl rollout undo deployment/hotel-reviews-api -n hotel-reviews-production

# Check rollout history
kubectl rollout history deployment/hotel-reviews-api -n hotel-reviews-production
```

## Support

For questions or issues:
- **Documentation**: Internal wiki
- **Slack**: #hotel-reviews-support
- **Email**: platform-team@hotel-reviews.com
- **Runbooks**: https://wiki.hotel-reviews.com/runbooks/

## Contributing

1. Test changes in staging environment
2. Follow security best practices
3. Update documentation
4. Peer review for production changes
5. Monitor deployment after changes