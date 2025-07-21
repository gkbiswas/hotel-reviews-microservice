# Global Terraform Variables
# Configuration parameters for multi-region deployment

###########################################
# Environment Configuration
###########################################

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "prod"
  
  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be one of: dev, staging, prod."
  }
}

variable "domain_name" {
  description = "Domain name for SSL certificates and DNS"
  type        = string
  
  validation {
    condition     = can(regex("^[a-z0-9.-]+\\.[a-z]{2,}$", var.domain_name))
    error_message = "Domain name must be a valid domain (e.g., example.com)."
  }
}

###########################################
# Primary Region Configuration
###########################################

variable "primary_db_instance_class" {
  description = "RDS instance class for primary region"
  type        = string
  default     = "db.r6g.xlarge"
  
  validation {
    condition     = can(regex("^db\\.", var.primary_db_instance_class))
    error_message = "DB instance class must start with 'db.'."
  }
}

variable "primary_db_allocated_storage" {
  description = "Allocated storage for primary RDS instance (GB)"
  type        = number
  default     = 200
  
  validation {
    condition     = var.primary_db_allocated_storage >= 20 && var.primary_db_allocated_storage <= 65536
    error_message = "DB allocated storage must be between 20 and 65536 GB."
  }
}

variable "primary_eks_node_groups" {
  description = "EKS node group configurations for primary region"
  type = map(object({
    instance_types        = list(string)
    capacity_type        = string
    disk_size           = number
    desired_size        = number
    max_size            = number
    min_size            = number
    labels              = map(string)
    taints              = list(object({
      key    = string
      value  = string
      effect = string
    }))
    bootstrap_arguments = string
  }))
  default = {
    general = {
      instance_types        = ["m6i.xlarge", "m6a.xlarge", "m5.xlarge"]
      capacity_type        = "ON_DEMAND"
      disk_size           = 100
      desired_size        = 5
      max_size            = 20
      min_size            = 3
      labels              = { role = "general", tier = "primary" }
      taints              = []
      bootstrap_arguments = "--container-runtime containerd --max-pods=50"
    }
    compute = {
      instance_types        = ["c6i.2xlarge", "c6a.2xlarge", "c5.2xlarge"]
      capacity_type        = "SPOT"
      disk_size           = 200
      desired_size        = 3
      max_size            = 30
      min_size            = 0
      labels              = { role = "compute", tier = "primary" }
      taints = [{
        key    = "compute"
        value  = "true"
        effect = "NO_SCHEDULE"
      }]
      bootstrap_arguments = "--container-runtime containerd --cpu-cfs-quota=true --max-pods=100"
    }
    memory = {
      instance_types        = ["r6i.xlarge", "r6a.xlarge", "r5.xlarge"]
      capacity_type        = "ON_DEMAND"
      disk_size           = 100
      desired_size        = 2
      max_size            = 10
      min_size            = 0
      labels              = { role = "memory", tier = "primary" }
      taints = [{
        key    = "memory-optimized"
        value  = "true"
        effect = "NO_SCHEDULE"
      }]
      bootstrap_arguments = "--container-runtime containerd --max-pods=30"
    }
  }
}

variable "primary_redis_node_type" {
  description = "Redis node type for primary region"
  type        = string
  default     = "cache.r7g.xlarge"
  
  validation {
    condition     = can(regex("^cache\\.", var.primary_redis_node_type))
    error_message = "Redis node type must start with 'cache.'."
  }
}

variable "primary_redis_num_cache_nodes" {
  description = "Number of Redis cache nodes for primary region"
  type        = number
  default     = 3
  
  validation {
    condition     = var.primary_redis_num_cache_nodes >= 1 && var.primary_redis_num_cache_nodes <= 6
    error_message = "Redis cache nodes must be between 1 and 6."
  }
}

###########################################
# Secondary Region Configuration
###########################################

variable "secondary_db_instance_class" {
  description = "RDS instance class for secondary region"
  type        = string
  default     = "db.r6g.large"
  
  validation {
    condition     = can(regex("^db\\.", var.secondary_db_instance_class))
    error_message = "DB instance class must start with 'db.'."
  }
}

variable "secondary_eks_node_groups" {
  description = "EKS node group configurations for secondary region"
  type = map(object({
    instance_types        = list(string)
    capacity_type        = string
    disk_size           = number
    desired_size        = number
    max_size            = number
    min_size            = number
    labels              = map(string)
    taints              = list(object({
      key    = string
      value  = string
      effect = string
    }))
    bootstrap_arguments = string
  }))
  default = {
    general = {
      instance_types        = ["m6i.large", "m6a.large", "m5.large"]
      capacity_type        = "ON_DEMAND"
      disk_size           = 50
      desired_size        = 3
      max_size            = 15
      min_size            = 2
      labels              = { role = "general", tier = "secondary" }
      taints              = []
      bootstrap_arguments = "--container-runtime containerd --max-pods=30"
    }
    compute = {
      instance_types        = ["c6i.xlarge", "c6a.xlarge", "c5.xlarge"]
      capacity_type        = "SPOT"
      disk_size           = 100
      desired_size        = 2
      max_size            = 20
      min_size            = 0
      labels              = { role = "compute", tier = "secondary" }
      taints = [{
        key    = "compute"
        value  = "true"
        effect = "NO_SCHEDULE"
      }]
      bootstrap_arguments = "--container-runtime containerd --cpu-cfs-quota=true --max-pods=50"
    }
  }
}

variable "secondary_redis_node_type" {
  description = "Redis node type for secondary region"
  type        = string
  default     = "cache.r7g.large"
  
  validation {
    condition     = can(regex("^cache\\.", var.secondary_redis_node_type))
    error_message = "Redis node type must start with 'cache.'."
  }
}

variable "secondary_redis_num_cache_nodes" {
  description = "Number of Redis cache nodes for secondary region"
  type        = number
  default     = 2
  
  validation {
    condition     = var.secondary_redis_num_cache_nodes >= 1 && var.secondary_redis_num_cache_nodes <= 6
    error_message = "Redis cache nodes must be between 1 and 6."
  }
}

###########################################
# Tertiary Region Configuration
###########################################

variable "tertiary_db_instance_class" {
  description = "RDS instance class for tertiary region"
  type        = string
  default     = "db.r6g.large"
  
  validation {
    condition     = can(regex("^db\\.", var.tertiary_db_instance_class))
    error_message = "DB instance class must start with 'db.'."
  }
}

variable "tertiary_eks_node_groups" {
  description = "EKS node group configurations for tertiary region"
  type = map(object({
    instance_types        = list(string)
    capacity_type        = string
    disk_size           = number
    desired_size        = number
    max_size            = number
    min_size            = number
    labels              = map(string)
    taints              = list(object({
      key    = string
      value  = string
      effect = string
    }))
    bootstrap_arguments = string
  }))
  default = {
    general = {
      instance_types        = ["m6i.large", "m6a.large", "m5.large"]
      capacity_type        = "SPOT"
      disk_size           = 50
      desired_size        = 2
      max_size            = 10
      min_size            = 1
      labels              = { role = "general", tier = "tertiary" }
      taints              = []
      bootstrap_arguments = "--container-runtime containerd --max-pods=30"
    }
    compute = {
      instance_types        = ["c6i.large", "c6a.large", "c5.large"]
      capacity_type        = "SPOT"
      disk_size           = 50
      desired_size        = 1
      max_size            = 10
      min_size            = 0
      labels              = { role = "compute", tier = "tertiary" }
      taints = [{
        key    = "compute"
        value  = "true"
        effect = "NO_SCHEDULE"
      }]
      bootstrap_arguments = "--container-runtime containerd --cpu-cfs-quota=true --max-pods=30"
    }
  }
}

variable "tertiary_redis_node_type" {
  description = "Redis node type for tertiary region"
  type        = string
  default     = "cache.r7g.large"
  
  validation {
    condition     = can(regex("^cache\\.", var.tertiary_redis_node_type))
    error_message = "Redis node type must start with 'cache.'."
  }
}

variable "tertiary_redis_num_cache_nodes" {
  description = "Number of Redis cache nodes for tertiary region"
  type        = number
  default     = 2
  
  validation {
    condition     = var.tertiary_redis_num_cache_nodes >= 1 && var.tertiary_redis_num_cache_nodes <= 6
    error_message = "Redis cache nodes must be between 1 and 6."
  }
}

###########################################
# Global Configuration
###########################################

variable "enable_global_accelerator" {
  description = "Enable AWS Global Accelerator for traffic distribution"
  type        = bool
  default     = true
}

variable "enable_cloudfront" {
  description = "Enable CloudFront CDN for global content delivery"
  type        = bool
  default     = true
}

variable "enable_route53_health_checks" {
  description = "Enable Route53 health checks for automated failover"
  type        = bool
  default     = true
}

variable "enable_cross_region_backups" {
  description = "Enable cross-region backups for disaster recovery"
  type        = bool
  default     = true
}

###########################################
# Security Configuration
###########################################

variable "enable_waf" {
  description = "Enable AWS WAF for web application protection"
  type        = bool
  default     = true
}

variable "enable_shield_advanced" {
  description = "Enable AWS Shield Advanced for DDoS protection"
  type        = bool
  default     = false
}

variable "enable_guarddduty" {
  description = "Enable AWS GuardDuty for threat detection"
  type        = bool
  default     = true
}

variable "enable_config" {
  description = "Enable AWS Config for compliance monitoring"
  type        = bool
  default     = true
}

###########################################
# Monitoring Configuration
###########################################

variable "enable_x_ray" {
  description = "Enable AWS X-Ray for distributed tracing"
  type        = bool
  default     = true
}

variable "enable_container_insights" {
  description = "Enable CloudWatch Container Insights"
  type        = bool
  default     = true
}

variable "cloudwatch_log_retention_days" {
  description = "CloudWatch logs retention period in days"
  type        = number
  default     = 30
  
  validation {
    condition     = contains([1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.cloudwatch_log_retention_days)
    error_message = "CloudWatch log retention days must be a valid value."
  }
}

###########################################
# Cost Optimization Configuration
###########################################

variable "enable_reserved_instances" {
  description = "Use reserved instances for cost optimization"
  type        = bool
  default     = true
}

variable "enable_savings_plans" {
  description = "Use savings plans for cost optimization"
  type        = bool
  default     = true
}

variable "enable_spot_instances" {
  description = "Use spot instances for non-critical workloads"
  type        = bool
  default     = true
}

variable "enable_lifecycle_policies" {
  description = "Enable S3 lifecycle policies for cost optimization"
  type        = bool
  default     = true
}

###########################################
# Compliance Configuration
###########################################

variable "enable_encryption_at_rest" {
  description = "Enable encryption at rest for all supported services"
  type        = bool
  default     = true
}

variable "enable_encryption_in_transit" {
  description = "Enable encryption in transit for all supported services"
  type        = bool
  default     = true
}

variable "enable_audit_trails" {
  description = "Enable CloudTrail for audit logging"
  type        = bool
  default     = true
}

variable "enable_vpc_flow_logs" {
  description = "Enable VPC Flow Logs for network monitoring"
  type        = bool
  default     = true
}

###########################################
# Disaster Recovery Configuration
###########################################

variable "rto_target_minutes" {
  description = "Recovery Time Objective target in minutes"
  type        = number
  default     = 5
  
  validation {
    condition     = var.rto_target_minutes >= 1 && var.rto_target_minutes <= 60
    error_message = "RTO target must be between 1 and 60 minutes."
  }
}

variable "rpo_target_minutes" {
  description = "Recovery Point Objective target in minutes"
  type        = number
  default     = 1
  
  validation {
    condition     = var.rpo_target_minutes >= 1 && var.rpo_target_minutes <= 60
    error_message = "RPO target must be between 1 and 60 minutes."
  }
}

variable "enable_automated_failover" {
  description = "Enable automated failover for critical components"
  type        = bool
  default     = true
}

variable "enable_backup_retention" {
  description = "Enable long-term backup retention"
  type        = bool
  default     = true
}