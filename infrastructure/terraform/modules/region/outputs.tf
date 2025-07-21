# Regional Infrastructure Outputs
# Export important resource identifiers and connection information

###########################################
# VPC and Networking Outputs
###########################################

output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "vpc_cidr_block" {
  description = "CIDR block of the VPC"
  value       = aws_vpc.main.cidr_block
}

output "private_subnet_ids" {
  description = "IDs of the private subnets"
  value       = aws_subnet.private[*].id
}

output "public_subnet_ids" {
  description = "IDs of the public subnets"
  value       = aws_subnet.public[*].id
}

output "database_subnet_ids" {
  description = "IDs of the database subnets"
  value       = aws_subnet.database[*].id
}

output "nat_gateway_ids" {
  description = "IDs of the NAT Gateways"
  value       = aws_nat_gateway.main[*].id
}

output "internet_gateway_id" {
  description = "ID of the Internet Gateway"
  value       = aws_internet_gateway.main.id
}

###########################################
# Security Group Outputs
###########################################

output "alb_security_group_id" {
  description = "ID of the ALB security group"
  value       = aws_security_group.alb.id
}

output "eks_cluster_security_group_id" {
  description = "ID of the EKS cluster security group"
  value       = aws_security_group.eks_cluster.id
}

output "eks_nodes_security_group_id" {
  description = "ID of the EKS nodes security group"
  value       = aws_security_group.eks_nodes.id
}

output "rds_security_group_id" {
  description = "ID of the RDS security group"
  value       = aws_security_group.rds.id
}

output "redis_security_group_id" {
  description = "ID of the Redis security group"
  value       = aws_security_group.redis.id
}

###########################################
# EKS Cluster Outputs
###########################################

output "eks_cluster_id" {
  description = "ID of the EKS cluster"
  value       = aws_eks_cluster.main.id
}

output "eks_cluster_arn" {
  description = "ARN of the EKS cluster"
  value       = aws_eks_cluster.main.arn
}

output "eks_cluster_endpoint" {
  description = "Endpoint for EKS control plane"
  value       = aws_eks_cluster.main.endpoint
}

output "eks_cluster_version" {
  description = "Version of the EKS cluster"
  value       = aws_eks_cluster.main.version
}

output "eks_cluster_security_group_id" {
  description = "Security group ID attached to the EKS cluster"
  value       = aws_eks_cluster.main.vpc_config[0].cluster_security_group_id
}

output "eks_cluster_oidc_issuer_url" {
  description = "The URL on the EKS cluster OIDC Issuer"
  value       = aws_eks_cluster.main.identity[0].oidc[0].issuer
}

output "eks_cluster_certificate_authority_data" {
  description = "Base64 encoded certificate data required to communicate with the cluster"
  value       = aws_eks_cluster.main.certificate_authority[0].data
}

output "eks_node_groups" {
  description = "EKS node group configurations"
  value = {
    for k, v in aws_eks_node_group.main : k => {
      arn           = v.arn
      status        = v.status
      capacity_type = v.capacity_type
      instance_types = v.instance_types
      scaling_config = v.scaling_config
    }
  }
}

output "eks_oidc_provider_arn" {
  description = "ARN of the OIDC Provider for IRSA"
  value       = aws_iam_openid_connect_provider.eks.arn
}

###########################################
# Load Balancer Outputs
###########################################

output "alb_id" {
  description = "ID of the Application Load Balancer"
  value       = aws_lb.main.id
}

output "alb_arn" {
  description = "ARN of the Application Load Balancer"
  value       = aws_lb.main.arn
}

output "alb_dns_name" {
  description = "DNS name of the Application Load Balancer"
  value       = aws_lb.main.dns_name
}

output "alb_zone_id" {
  description = "Zone ID of the Application Load Balancer"
  value       = aws_lb.main.zone_id
}

output "alb_target_groups" {
  description = "ALB target group ARNs"
  value = {
    api_gateway         = aws_lb_target_group.api_gateway.arn
    user_service       = aws_lb_target_group.user_service.arn
    hotel_service      = aws_lb_target_group.hotel_service.arn
    review_service     = aws_lb_target_group.review_service.arn
    analytics_service  = aws_lb_target_group.analytics_service.arn
    notification_service = aws_lb_target_group.notification_service.arn
    file_service       = aws_lb_target_group.file_service.arn
    search_service     = aws_lb_target_group.search_service.arn
  }
}

###########################################
# Database Outputs
###########################################

output "db_identifier" {
  description = "RDS instance identifier"
  value       = var.is_primary ? aws_db_instance.primary[0].identifier : aws_db_instance.replica[0].identifier
}

output "db_endpoint" {
  description = "RDS instance endpoint"
  value       = var.is_primary ? aws_db_instance.primary[0].endpoint : aws_db_instance.replica[0].endpoint
}

output "db_port" {
  description = "RDS instance port"
  value       = var.is_primary ? aws_db_instance.primary[0].port : aws_db_instance.replica[0].port
}

output "db_name" {
  description = "RDS database name"
  value       = var.is_primary ? aws_db_instance.primary[0].db_name : "hotel_reviews"
}

output "db_username" {
  description = "RDS database username"
  value       = var.is_primary ? aws_db_instance.primary[0].username : "readonly_user"
  sensitive   = true
}

output "db_kms_key_id" {
  description = "KMS key ID used for RDS encryption"
  value       = var.is_primary ? aws_kms_key.rds[0].arn : null
}

output "db_subnet_group_name" {
  description = "DB subnet group name"
  value       = aws_db_subnet_group.main.name
}

output "db_parameter_group_name" {
  description = "DB parameter group name"
  value       = aws_db_parameter_group.main.name
}

output "db_secret_arn" {
  description = "ARN of the database secret in AWS Secrets Manager"
  value       = aws_secretsmanager_secret.db_password.arn
}

###########################################
# Redis Cache Outputs
###########################################

output "redis_cluster_id" {
  description = "ID of the Redis cluster"
  value       = aws_elasticache_replication_group.main.id
}

output "redis_primary_endpoint" {
  description = "Address of the Redis primary endpoint"
  value       = aws_elasticache_replication_group.main.configuration_endpoint_address
}

output "redis_port" {
  description = "Port of the Redis cluster"
  value       = aws_elasticache_replication_group.main.port
}

output "redis_auth_token_secret_arn" {
  description = "ARN of the Redis auth token secret in AWS Secrets Manager"
  value       = aws_secretsmanager_secret.redis_auth_token.arn
}

###########################################
# SSL Certificate Outputs
###########################################

output "ssl_certificate_arn" {
  description = "ARN of the SSL certificate"
  value       = aws_acm_certificate.main.arn
}

output "ssl_certificate_domain_name" {
  description = "Domain name of the SSL certificate"
  value       = aws_acm_certificate.main.domain_name
}

###########################################
# S3 Bucket Outputs
###########################################

output "alb_logs_bucket" {
  description = "S3 bucket for ALB access logs"
  value       = aws_s3_bucket.alb_logs.bucket
}

output "alb_logs_bucket_arn" {
  description = "ARN of the S3 bucket for ALB access logs"
  value       = aws_s3_bucket.alb_logs.arn
}

###########################################
# Region Configuration Outputs
###########################################

output "region" {
  description = "AWS region"
  value       = local.region_name
}

output "region_code" {
  description = "Region code"
  value       = local.region_code
}

output "availability_zones" {
  description = "List of availability zones"
  value       = local.azs
}

output "is_primary" {
  description = "Whether this is the primary region"
  value       = var.is_primary
}

###########################################
# CloudWatch Outputs
###########################################

output "cloudwatch_log_groups" {
  description = "CloudWatch log groups created"
  value = {
    eks_cluster = aws_cloudwatch_log_group.eks_cluster.name
    redis_slow  = aws_cloudwatch_log_group.redis_slow.name
  }
}

###########################################
# IAM Role Outputs
###########################################

output "eks_cluster_role_arn" {
  description = "ARN of the EKS cluster IAM role"
  value       = aws_iam_role.eks_cluster.arn
}

output "eks_nodes_role_arn" {
  description = "ARN of the EKS nodes IAM role"
  value       = aws_iam_role.eks_nodes.arn
}

output "rds_monitoring_role_arn" {
  description = "ARN of the RDS monitoring IAM role"
  value       = aws_iam_role.rds_monitoring.arn
}