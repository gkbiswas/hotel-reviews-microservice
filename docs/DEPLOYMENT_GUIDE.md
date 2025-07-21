# 🚀 Multi-Region Deployment Guide

**Version**: 2.0  
**Last Updated**: 2025-01-21  
**Target Environment**: Production Multi-Region

---

## 📋 **Prerequisites**

### **Required Tools**
```bash
# Install required tools
terraform >= 1.0
aws-cli >= 2.0
kubectl >= 1.28
docker >= 20.0
helm >= 3.0
```

### **AWS Permissions**
- IAM permissions for multi-region resource creation
- Route53 hosted zone for domain management
- S3 bucket for Terraform state storage

### **Environment Setup**
```bash
# Configure AWS CLI
aws configure --profile production

# Set environment variables
export AWS_PROFILE=production
export TF_VAR_domain_name="your-domain.com"
export TF_VAR_environment="prod"
```

---

## 🌍 **Multi-Region Infrastructure Deployment**

### **Step 1: Initialize Terraform Backend**

```bash
# Create S3 bucket for Terraform state
aws s3 mb s3://hotel-reviews-terraform-state --region us-east-1

# Create DynamoDB table for state locking
aws dynamodb create-table \
    --table-name terraform-locks \
    --attribute-definitions AttributeName=LockID,AttributeType=S \
    --key-schema AttributeName=LockID,KeyType=HASH \
    --provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5 \
    --region us-east-1
```

### **Step 2: Deploy Global Infrastructure**

```bash
cd infrastructure/terraform

# Initialize Terraform
terraform init

# Plan deployment
terraform plan -var-file="prod.tfvars"

# Deploy infrastructure
terraform apply -var-file="prod.tfvars"
```

### **Step 3: Verify Regional Deployments**

```bash
# Check primary region (US-East-1)
aws eks describe-cluster --name hotel-reviews-use1-eks --region us-east-1

# Check secondary region (EU-West-1)  
aws eks describe-cluster --name hotel-reviews-euw1-eks --region eu-west-1

# Check tertiary region (AP-Southeast-1)
aws eks describe-cluster --name hotel-reviews-apse1-eks --region ap-southeast-1
```

---

## 🔧 **Kubernetes Configuration**

### **Step 1: Configure kubectl for Each Region**

```bash
# Primary region
aws eks update-kubeconfig --region us-east-1 --name hotel-reviews-use1-eks --alias primary

# Secondary region
aws eks update-kubeconfig --region eu-west-1 --name hotel-reviews-euw1-eks --alias secondary

# Tertiary region
aws eks update-kubeconfig --region ap-southeast-1 --name hotel-reviews-apse1-eks --alias tertiary
```

### **Step 2: Deploy Kubernetes Addons**

```bash
# Deploy AWS Load Balancer Controller (per region)
helm repo add eks https://aws.github.io/eks-charts
helm repo update

# Primary region
kubectl --context primary apply -f k8s/addons/aws-load-balancer-controller.yaml

# Secondary region
kubectl --context secondary apply -f k8s/addons/aws-load-balancer-controller.yaml

# Tertiary region  
kubectl --context tertiary apply -f k8s/addons/aws-load-balancer-controller.yaml
```

---

## 🐳 **Application Deployment**

### **Step 1: Build and Push Container Images**

```bash
# Build application image
docker build -t hotel-reviews:latest .

# Tag for ECR repositories (per region)
docker tag hotel-reviews:latest 123456789012.dkr.ecr.us-east-1.amazonaws.com/hotel-reviews:latest
docker tag hotel-reviews:latest 123456789012.dkr.ecr.eu-west-1.amazonaws.com/hotel-reviews:latest
docker tag hotel-reviews:latest 123456789012.dkr.ecr.ap-southeast-1.amazonaws.com/hotel-reviews:latest

# Push to ECR (per region)
aws ecr get-login-password --region us-east-1 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com
docker push 123456789012.dkr.ecr.us-east-1.amazonaws.com/hotel-reviews:latest

aws ecr get-login-password --region eu-west-1 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.eu-west-1.amazonaws.com
docker push 123456789012.dkr.ecr.eu-west-1.amazonaws.com/hotel-reviews:latest

aws ecr get-login-password --region ap-southeast-1 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.ap-southeast-1.amazonaws.com
docker push 123456789012.dkr.ecr.ap-southeast-1.amazonaws.com/hotel-reviews:latest
```

### **Step 2: Deploy Application Manifests**

```bash
# Deploy to primary region
kubectl --context primary apply -f k8s/manifests/primary/

# Deploy to secondary region
kubectl --context secondary apply -f k8s/manifests/secondary/

# Deploy to tertiary region
kubectl --context tertiary apply -f k8s/manifests/tertiary/
```

---

## 📊 **Database Setup**

### **Step 1: Run Database Migrations**

```bash
# Connect to primary database
go run cmd/migrate/main.go up

# Verify replica sync
psql -h hotel-reviews-euw1-replica.cluster-abc.eu-west-1.rds.amazonaws.com -U readonly_user -d hotel_reviews -c "SELECT COUNT(*) FROM reviews;"
psql -h hotel-reviews-apse1-replica.cluster-def.ap-southeast-1.rds.amazonaws.com -U readonly_user -d hotel_reviews -c "SELECT COUNT(*) FROM reviews;"
```

### **Step 2: Configure Read/Write Routing**

```yaml
# Application configuration for database routing
database:
  primary:
    host: hotel-reviews-use1-primary.cluster-xyz.us-east-1.rds.amazonaws.com
    port: 5432
    database: hotel_reviews
    username: hotelreviews_admin
    
  read_replicas:
    - host: hotel-reviews-euw1-replica.cluster-abc.eu-west-1.rds.amazonaws.com
      region: eu-west-1
    - host: hotel-reviews-apse1-replica.cluster-def.ap-southeast-1.rds.amazonaws.com
      region: ap-southeast-1
```

---

## 🔒 **Security Configuration**

### **Step 1: SSL Certificate Validation**

```bash
# Validate global certificate
aws acm describe-certificate --certificate-arn arn:aws:acm:us-east-1:123456789012:certificate/abc-def-ghi --region us-east-1

# Validate regional certificates
aws acm describe-certificate --certificate-arn arn:aws:acm:us-east-1:123456789012:certificate/use1-cert --region us-east-1
aws acm describe-certificate --certificate-arn arn:aws:acm:eu-west-1:123456789012:certificate/euw1-cert --region eu-west-1
aws acm describe-certificate --certificate-arn arn:aws:acm:ap-southeast-1:123456789012:certificate/apse1-cert --region ap-southeast-1
```

### **Step 2: Configure WAF (Pending Implementation)**

```bash
# WAF configuration will be added in next phase
# aws wafv2 create-web-acl --scope CLOUDFRONT --region us-east-1
```

---

## 📈 **Monitoring Setup**

### **Step 1: CloudWatch Dashboards**

```bash
# Deploy CloudWatch dashboards
aws cloudwatch put-dashboard --dashboard-name "HotelReviews-Global" --dashboard-body file://monitoring/cloudwatch-dashboard.json --region us-east-1
```

### **Step 2: Health Check Validation**

```bash
# Test Global Accelerator endpoint
curl -I https://hotel-reviews-global-accelerator.awsglobalaccelerator.com/health

# Test regional endpoints
curl -I https://use1.your-domain.com/health
curl -I https://euw1.your-domain.com/health  
curl -I https://apse1.your-domain.com/health
```

---

## 🧪 **Testing & Validation**

### **Step 1: Infrastructure Tests**

```bash
# Run Terraform validation
terraform validate
terraform plan -detailed-exitcode

# Run infrastructure tests
cd tests/infrastructure
go test ./... -v
```

### **Step 2: Application Health Checks**

```bash
# Primary region health check
curl https://use1.your-domain.com/api/health

# Secondary region health check  
curl https://euw1.your-domain.com/api/health

# Tertiary region health check
curl https://apse1.your-domain.com/api/health
```

### **Step 3: Database Connectivity Tests**

```bash
# Test primary database writes
curl -X POST https://use1.your-domain.com/api/reviews -H "Content-Type: application/json" -d '{"hotel_id":"test","rating":5,"comment":"test"}'

# Test replica reads
curl https://euw1.your-domain.com/api/reviews/search?query=test
curl https://apse1.your-domain.com/api/reviews/search?query=test
```

---

## 🚨 **Rollback Procedures**

### **Emergency Rollback**

```bash
# Rollback to previous Terraform state
terraform apply -target=module.primary_region -var="traffic_percentage=0"

# Scale down problematic regions
kubectl --context secondary scale deployment hotel-reviews --replicas=0
kubectl --context tertiary scale deployment hotel-reviews --replicas=0
```

### **Gradual Rollback**

```bash
# Update Global Accelerator traffic distribution
aws globalaccelerator update-endpoint-group \
    --endpoint-group-arn arn:aws:globalaccelerator::123456789012:accelerator/abc/listener/def/endpoint-group/ghi \
    --traffic-dial-percentage 0
```

---

## 📋 **Deployment Checklist**

### **Pre-Deployment**
- [ ] AWS credentials configured
- [ ] Domain DNS configured
- [ ] Terraform state backend ready
- [ ] Container images built and pushed
- [ ] Database backup completed

### **Deployment**
- [ ] Terraform infrastructure deployed successfully
- [ ] EKS clusters accessible
- [ ] Database migrations completed
- [ ] Application pods running
- [ ] Health checks passing

### **Post-Deployment**
- [ ] Global Accelerator routing tested
- [ ] SSL certificates validated
- [ ] Monitoring dashboards configured
- [ ] Alerting configured
- [ ] Documentation updated

---

## 🔗 **Related Documentation**

- [Multi-Region Architecture](./multi-region-architecture.md)
- [Infrastructure Status](./INFRASTRUCTURE_STATUS.md)
- [Monitoring Runbook](./runbooks/monitoring-runbook.md)
- [Incident Response](./runbooks/incident-response-runbook.md)

---

## 📞 **Support**

**Infrastructure Team**: Goutam Kumar Biswas  
**Email**: gkbiswas@gmail.com  
**GitHub**: @gkbiswas