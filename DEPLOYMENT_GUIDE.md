# Hotel Reviews Microservice - Production Deployment Guide

This guide provides comprehensive instructions for deploying the Hotel Reviews microservice to production using a blue-green deployment strategy with full CI/CD automation.

## Overview

The deployment system includes:
- **Blue-Green Deployment**: Zero-downtime deployments with automatic traffic switching
- **Database Migration Management**: Automated migrations with rollback capabilities
- **Infrastructure as Code**: Terraform modules for AWS infrastructure
- **Comprehensive Monitoring**: CloudWatch alarms, dashboards, and health checks
- **Automated Rollbacks**: Failure detection and automatic rollback mechanisms
- **Security**: Vulnerability scanning, secret management, and compliance logging

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   GitHub        │    │   AWS ECS       │    │   AWS RDS       │
│   Actions       │───▶│   Blue/Green    │───▶│   PostgreSQL    │
│   Pipeline      │    │   Services      │    │   Database      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Terraform     │    │   Application   │    │   CloudWatch    │
│   Infrastructure│    │   Load Balancer │    │   Monitoring    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## Quick Start

### 1. Prerequisites

- AWS CLI configured with appropriate permissions
- Terraform >= 1.0
- PostgreSQL client tools
- Docker and Docker Compose
- GitHub repository with Actions enabled

### 2. Environment Setup

```bash
# Clone the repository
git clone https://github.com/gkbiswas/hotel-reviews-microservice.git
cd hotel-reviews-microservice

# Set up environment variables
export AWS_REGION=us-east-1
export ENVIRONMENT=staging  # or production
export DATABASE_PASSWORD=your-secure-password
```

### 3. Infrastructure Deployment

```bash
# Initialize Terraform
cd terraform/environments/staging
terraform init

# Plan and apply infrastructure
terraform plan
terraform apply
```

### 4. Database Setup

```bash
# Run database migrations
./scripts/migrations/migrate.sh --environment staging

# Verify migration status
./scripts/migrations/migrate.sh --status --environment staging
```

### 5. Application Deployment

```bash
# Deploy application using GitHub Actions
git push origin main

# Or deploy manually
./scripts/deployment/blue-green-deploy.sh \
  --image ghcr.io/gkbiswas/hotel-reviews-microservice:latest \
  --environment staging \
  --deployment-type blue-green
```

## Deployment Workflow

### GitHub Actions Pipeline

The deployment pipeline (`.github/workflows/deploy.yml`) includes:

1. **Security Scanning**: Vulnerability assessment and dependency checks
2. **Build & Test**: Code compilation, unit tests, and coverage analysis
3. **Infrastructure Validation**: Terraform plan and apply
4. **Pre-deployment Checks**: Database connectivity and dependency validation
5. **Database Migration**: Schema updates with backup and rollback support
6. **Blue-Green Deployment**: Zero-downtime application deployment
7. **Health Checks**: Post-deployment validation and smoke tests
8. **Traffic Switching**: Gradual or immediate traffic migration
9. **Monitoring**: Continuous health monitoring and alerting

### Manual Deployment

For manual deployments or testing:

```bash
# 1. Run pre-deployment checks
./scripts/health-checks/database-check.sh
./scripts/health-checks/dependency-check.sh

# 2. Create database backup
./scripts/migrations/backup-database.sh --environment staging

# 3. Run database migrations
./scripts/migrations/migrate.sh --environment staging

# 4. Deploy application
./scripts/deployment/blue-green-deploy.sh \
  --image your-image-uri \
  --environment staging \
  --deployment-type blue-green

# 5. Switch traffic
./scripts/deployment/traffic-switch.sh \
  --target-color green \
  --current-color blue \
  --environment staging \
  --strategy gradual

# 6. Run post-deployment validation
./scripts/health-checks/post-deployment-check.sh \
  --target-color green \
  --environment staging
```

## Deployment Strategies

### Blue-Green Deployment (Default)

- **Zero Downtime**: Complete environment switch with no service interruption
- **Quick Rollback**: Instant rollback by switching traffic back
- **Full Validation**: Complete testing before traffic switch

```bash
# Deploy to green environment
./scripts/deployment/blue-green-deploy.sh \
  --image new-image \
  --environment production \
  --deployment-type blue-green

# Switch traffic immediately
./scripts/deployment/traffic-switch.sh \
  --strategy immediate \
  --target-color green \
  --current-color blue
```

### Canary Deployment

- **Gradual Rollout**: Start with small percentage of traffic
- **Risk Mitigation**: Monitor metrics before full deployment
- **A/B Testing**: Compare performance between versions

```bash
# Deploy canary version
./scripts/deployment/blue-green-deploy.sh \
  --image new-image \
  --environment production \
  --deployment-type canary

# Start with 10% traffic
./scripts/deployment/traffic-switch.sh \
  --strategy canary \
  --canary-percentage 10 \
  --target-color green
```

### Rolling Deployment

- **Gradual Update**: Update instances one by one
- **Resource Efficient**: No additional infrastructure required
- **Progressive Rollout**: Automated health checks at each step

```bash
# Rolling deployment
./scripts/deployment/blue-green-deploy.sh \
  --image new-image \
  --environment production \
  --deployment-type rolling
```

## Database Management

### Migration System

The migration system provides:
- **Version Control**: Track and manage schema changes
- **Rollback Support**: Automated rollback capabilities
- **Backup Integration**: Pre-migration backups
- **Validation**: Schema integrity checks

```bash
# Create a new migration
cat > migrations/004_add_user_preferences.sql << 'EOF'
-- Migration: Add user preferences table
CREATE TABLE user_preferences (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    preferences JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ROLLBACK START
-- DROP TABLE user_preferences;
-- ROLLBACK END
EOF

# Apply migration
./scripts/migrations/migrate.sh --environment staging

# Rollback if needed
./scripts/migrations/rollback.sh --steps 1 --environment staging
```

### Backup and Recovery

```bash
# Create full database backup
./scripts/migrations/backup-database.sh \
  --environment production \
  --type full \
  --s3-bucket hotel-reviews-backups

# List available backups
./scripts/migrations/backup-database.sh --list --environment production

# Restore from backup (example)
psql -h db-host -U username -d database_name < backup_file.sql
```

## Monitoring and Alerting

### CloudWatch Alarms

The system includes comprehensive monitoring:

- **Application Metrics**: CPU, Memory, Response Time
- **Database Metrics**: Connections, Disk Space, Query Performance
- **Infrastructure Metrics**: Load Balancer Health, ECS Service Status
- **Custom Metrics**: Application Errors, Slow Queries

### Health Checks

```bash
# Application health check
curl -f http://your-load-balancer/api/v1/health

# Database health check
./scripts/health-checks/database-check.sh --environment production

# Full system health check
./scripts/health-checks/post-deployment-check.sh --environment production
```

### Dashboards

Access monitoring dashboards:
- **CloudWatch Dashboard**: AWS Console → CloudWatch → Dashboards
- **Application Logs**: CloudWatch Logs → `/ecs/hotel-reviews-service/app`
- **Database Metrics**: RDS Performance Insights

## Security

### Secret Management

Secrets are stored in AWS Systems Manager Parameter Store:

```bash
# Store database password
aws ssm put-parameter \
  --name "/production/hotel-reviews-service/database/password" \
  --value "your-secure-password" \
  --type "SecureString"

# Retrieve secret in application
aws ssm get-parameter \
  --name "/production/hotel-reviews-service/database/password" \
  --with-decryption \
  --query 'Parameter.Value' \
  --output text
```

### Vulnerability Scanning

- **Container Scanning**: Trivy scans for vulnerabilities
- **Dependency Scanning**: npm audit and Go security checks
- **Code Scanning**: Static analysis with security rules
- **Infrastructure Scanning**: Terraform security validation

## Rollback Procedures

### Automatic Rollback

The system automatically rolls back on:
- Health check failures
- High error rates
- Performance degradation
- Infrastructure issues

### Manual Rollback

```bash
# Rollback deployment
./scripts/deployment/rollback.sh \
  --environment production \
  --current-color blue \
  --target-color green \
  --reason "performance_issues"

# Rollback database migrations
./scripts/migrations/rollback.sh \
  --steps 2 \
  --environment production \
  --force
```

## Troubleshooting

### Common Issues

1. **Deployment Fails**
   ```bash
   # Check ECS service status
   aws ecs describe-services --cluster hotel-reviews-production-cluster \
     --services hotel-reviews-service-green
   
   # Check application logs
   aws logs tail /ecs/hotel-reviews-service/app --follow
   ```

2. **Database Connection Issues**
   ```bash
   # Test database connectivity
   ./scripts/health-checks/database-check.sh --environment production
   
   # Check database logs
   aws rds describe-db-log-files --db-instance-identifier hotel-reviews-production-db
   ```

3. **Traffic Switching Problems**
   ```bash
   # Check load balancer configuration
   aws elbv2 describe-listeners --load-balancer-arn your-lb-arn
   
   # Verify target group health
   aws elbv2 describe-target-health --target-group-arn your-tg-arn
   ```

### Recovery Procedures

1. **Application Recovery**
   ```bash
   # Switch traffic back to known good version
   ./scripts/deployment/traffic-switch.sh \
     --target-color blue \
     --current-color green \
     --strategy immediate
   
   # Scale up healthy service
   aws ecs update-service \
     --cluster hotel-reviews-production-cluster \
     --service hotel-reviews-service-blue \
     --desired-count 3
   ```

2. **Database Recovery**
   ```bash
   # Restore from backup
   ./scripts/migrations/backup-database.sh \
     --restore \
     --backup-file production_full_20240101_120000.sql
   
   # Verify data integrity
   psql -h db-host -U username -d database -c "SELECT COUNT(*) FROM hotel_reviews;"
   ```

## Production Considerations

### Performance Optimization

- **Auto Scaling**: Configured based on CPU and memory metrics
- **Connection Pooling**: Database connection optimization
- **Caching**: Redis integration for frequently accessed data
- **CDN**: CloudFront for static assets

### Security Best Practices

- **Least Privilege**: IAM roles with minimal required permissions
- **Network Security**: VPC with private subnets and security groups
- **Data Encryption**: At rest and in transit encryption
- **Regular Updates**: Automated security patching

### Cost Management

- **Resource Sizing**: Right-sized instances based on actual usage
- **Spot Instances**: For non-critical workloads
- **Storage Optimization**: Lifecycle policies for logs and backups
- **Monitoring**: Cost tracking and alerts

## Support and Maintenance

### Regular Maintenance

1. **Weekly**: Review monitoring dashboards and alerts
2. **Monthly**: Update dependencies and security patches
3. **Quarterly**: Disaster recovery testing
4. **Annually**: Security audit and compliance review

### Contact Information

- **Development Team**: dev@company.com
- **Operations Team**: ops@company.com
- **Emergency Contact**: oncall@company.com

## Appendix

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `AWS_REGION` | AWS region | `us-east-1` |
| `ENVIRONMENT` | Deployment environment | `staging` or `production` |
| `DATABASE_PASSWORD` | Database password | `secure-password-123` |
| `SLACK_WEBHOOK` | Slack notification URL | `https://hooks.slack.com/...` |

### Required AWS Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ecs:*",
        "elbv2:*",
        "rds:*",
        "ssm:*",
        "cloudwatch:*",
        "logs:*",
        "s3:*"
      ],
      "Resource": "*"
    }
  ]
}
```

### Useful Commands

```bash
# View deployment status
aws ecs describe-services --cluster hotel-reviews-production-cluster \
  --services hotel-reviews-service-blue hotel-reviews-service-green

# Check application health
curl -s http://your-load-balancer/api/v1/health | jq '.'

# View recent logs
aws logs tail /ecs/hotel-reviews-service/app --since 1h

# Check database performance
aws rds describe-db-instances --db-instance-identifier hotel-reviews-production-db

# Monitor costs
aws ce get-cost-and-usage --time-period Start=2024-01-01,End=2024-01-31 \
  --granularity MONTHLY --metrics BlendedCost
```

This deployment guide provides a comprehensive overview of the production deployment system. For specific technical details, refer to the individual script files and Terraform modules in the repository.