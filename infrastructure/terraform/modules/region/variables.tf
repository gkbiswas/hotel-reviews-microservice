# Regional Infrastructure Variables
# Configuration parameters for regional deployment

###########################################
# General Configuration
###########################################

variable "project_name" {
  description = "Name of the project"
  type        = string
  default     = "hotel-reviews"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "prod"
}

variable "region_config" {
  description = "Region configuration including name, code, and VPC CIDR"
  type = object({
    name     = string
    code     = string
    vpc_cidr = string
  })
}

variable "is_primary" {
  description = "Whether this region is the primary region"
  type        = bool
  default     = false
}

variable "tags" {
  description = "A map of tags to assign to resources"
  type        = map(string)
  default     = {}
}

variable "domain_name" {
  description = "Domain name for SSL certificate and DNS"
  type        = string
}

###########################################
# Database Configuration
###########################################

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.r6g.large"
}

variable "db_allocated_storage" {
  description = "Allocated storage for RDS instance (GB)"
  type        = number
  default     = 100
}

variable "primary_db_identifier" {
  description = "Identifier of the primary database (for read replicas)"
  type        = string
  default     = null
}

variable "primary_db_kms_key_id" {
  description = "KMS key ID of the primary database (for read replicas)"
  type        = string
  default     = null
}

###########################################
# EKS Configuration
###########################################

variable "eks_cluster_version" {
  description = "Kubernetes version for the EKS cluster"
  type        = string
  default     = "1.28"
}

variable "eks_public_access_cidrs" {
  description = "List of CIDR blocks that can access the EKS public API server endpoint"
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "eks_node_groups" {
  description = "EKS node group configurations"
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
      max_size            = 10
      min_size            = 2
      labels              = { role = "general" }
      taints              = []
      bootstrap_arguments = "--container-runtime containerd"
    }
    compute = {
      instance_types        = ["c6i.xlarge", "c6a.xlarge", "c5.xlarge"]
      capacity_type        = "SPOT"
      disk_size           = 100
      desired_size        = 2
      max_size            = 20
      min_size            = 0
      labels              = { role = "compute" }
      taints = [{
        key    = "compute"
        value  = "true"
        effect = "NO_SCHEDULE"
      }]
      bootstrap_arguments = "--container-runtime containerd --cpu-cfs-quota=true"
    }
  }
}

# EKS Add-on Versions
variable "vpc_cni_version" {
  description = "Version of the VPC CNI add-on"
  type        = string
  default     = "v1.15.1-eksbuild.1"
}

variable "coredns_version" {
  description = "Version of the CoreDNS add-on"
  type        = string
  default     = "v1.10.1-eksbuild.6"
}

variable "kube_proxy_version" {
  description = "Version of the kube-proxy add-on"
  type        = string
  default     = "v1.28.2-eksbuild.2"
}

variable "ebs_csi_version" {
  description = "Version of the EBS CSI driver add-on"
  type        = string
  default     = "v1.24.0-eksbuild.1"
}

###########################################
# Redis Configuration
###########################################

variable "redis_node_type" {
  description = "ElastiCache Redis node type"
  type        = string
  default     = "cache.r7g.large"
}

variable "redis_num_cache_nodes" {
  description = "Number of cache nodes in the Redis cluster"
  type        = number
  default     = 3
}

###########################################
# Monitoring Configuration
###########################################

variable "enable_detailed_monitoring" {
  description = "Enable detailed monitoring for resources"
  type        = bool
  default     = true
}

variable "log_retention_days" {
  description = "Number of days to retain CloudWatch logs"
  type        = number
  default     = 30
}

###########################################
# Security Configuration
###########################################

variable "enable_encryption" {
  description = "Enable encryption for all supported resources"
  type        = bool
  default     = true
}

variable "kms_key_deletion_window" {
  description = "KMS key deletion window in days"
  type        = number
  default     = 7
}

###########################################
# Load Balancer Configuration
###########################################

variable "enable_deletion_protection" {
  description = "Enable deletion protection for load balancer"
  type        = bool
  default     = true
}

variable "alb_idle_timeout" {
  description = "ALB idle timeout in seconds"
  type        = number
  default     = 60
}

###########################################
# High Availability Configuration
###########################################

variable "enable_multi_az" {
  description = "Enable Multi-AZ deployment for databases"
  type        = bool
  default     = true
}

variable "backup_retention_period" {
  description = "Backup retention period in days"
  type        = number
  default     = 30
}

###########################################
# Cost Optimization Configuration
###########################################

variable "enable_spot_instances" {
  description = "Enable spot instances for non-critical workloads"
  type        = bool
  default     = true
}

variable "enable_storage_optimization" {
  description = "Enable storage optimization features"
  type        = bool
  default     = true
}

###########################################
# Compliance Configuration
###########################################

variable "enable_audit_logging" {
  description = "Enable audit logging for compliance"
  type        = bool
  default     = true
}

variable "enable_access_logging" {
  description = "Enable access logging for load balancers"
  type        = bool
  default     = true
}

###########################################
# Networking Configuration
###########################################

variable "enable_nat_gateway_per_az" {
  description = "Create a NAT Gateway per availability zone"
  type        = bool
  default     = true
}

variable "enable_dns_hostnames" {
  description = "Enable DNS hostnames in VPC"
  type        = bool
  default     = true
}

variable "enable_dns_support" {
  description = "Enable DNS support in VPC"
  type        = bool
  default     = true
}