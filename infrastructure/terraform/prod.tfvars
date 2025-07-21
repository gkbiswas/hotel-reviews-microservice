# Production Environment Configuration
# Multi-Region Hotel Reviews Infrastructure

environment = "prod"
domain_name = "hotelreviews.com"

# Global Features
enable_global_accelerator = true
enable_cloudfront = true
enable_route53_health_checks = true
enable_cross_region_backups = true

# Security Configuration
enable_waf = true
enable_shield_advanced = false  # Requires subscription
enable_guarddduty = true
enable_config = true
enable_encryption_at_rest = true
enable_encryption_in_transit = true
enable_audit_trails = true
enable_vpc_flow_logs = true

# Monitoring Configuration
enable_x_ray = true
enable_container_insights = true
cloudwatch_log_retention_days = 30

# Cost Optimization
enable_reserved_instances = true
enable_savings_plans = true
enable_spot_instances = true
enable_lifecycle_policies = true

# Disaster Recovery
rto_target_minutes = 5
rpo_target_minutes = 1
enable_automated_failover = true
enable_backup_retention = true

###########################################
# Primary Region Configuration (US-East-1)
###########################################

primary_db_instance_class = "db.r6g.xlarge"
primary_db_allocated_storage = 200

primary_redis_node_type = "cache.r7g.xlarge"
primary_redis_num_cache_nodes = 3

primary_eks_node_groups = {
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

###########################################
# Secondary Region Configuration (EU-West-1)
###########################################

secondary_db_instance_class = "db.r6g.large"
secondary_redis_node_type = "cache.r7g.large"
secondary_redis_num_cache_nodes = 2

secondary_eks_node_groups = {
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

###########################################
# Tertiary Region Configuration (AP-Southeast-1)
###########################################

tertiary_db_instance_class = "db.r6g.large"
tertiary_redis_node_type = "cache.r7g.large"
tertiary_redis_num_cache_nodes = 2

tertiary_eks_node_groups = {
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