# Database Infrastructure
# RDS PostgreSQL with Multi-AZ for primary region, read replicas for secondary regions

###########################################
# Database Subnet Group
###########################################

resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-${local.region_code}-db-subnet-group"
  subnet_ids = aws_subnet.database[*].id
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-db-subnet-group"
  })
}

###########################################
# KMS Key for Database Encryption
###########################################

resource "aws_kms_key" "rds" {
  count = var.is_primary ? 1 : 0
  
  description             = "KMS key for RDS encryption in ${local.region_name}"
  deletion_window_in_days = 7
  enable_key_rotation     = true
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-rds-kms"
  })
}

resource "aws_kms_alias" "rds" {
  count = var.is_primary ? 1 : 0
  
  name          = "alias/${var.project_name}-${local.region_code}-rds"
  target_key_id = aws_kms_key.rds[0].key_id
}

###########################################
# Database Parameter Group
###########################################

resource "aws_db_parameter_group" "main" {
  family = "postgres15"
  name   = "${var.project_name}-${local.region_code}-postgres15"
  
  # Performance optimization parameters
  parameter {
    name  = "shared_preload_libraries"
    value = "pg_stat_statements,auto_explain"
  }
  
  parameter {
    name  = "log_statement"
    value = "all"
  }
  
  parameter {
    name  = "log_min_duration_statement"
    value = "1000"  # Log queries taking longer than 1 second
  }
  
  parameter {
    name  = "auto_explain.log_min_duration"
    value = "1000"
  }
  
  parameter {
    name  = "auto_explain.log_analyze"
    value = "1"
  }
  
  parameter {
    name  = "auto_explain.log_buffers"
    value = "1"
  }
  
  parameter {
    name         = "max_connections"
    value        = var.is_primary ? "200" : "100"
    apply_method = "pending-reboot"
  }
  
  parameter {
    name  = "work_mem"
    value = "16384"  # 16MB
  }
  
  parameter {
    name  = "maintenance_work_mem"
    value = "262144"  # 256MB
  }
  
  parameter {
    name  = "effective_cache_size"
    value = var.is_primary ? "2097152" : "1048576"  # 2GB for primary, 1GB for replicas
  }
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-postgres15-params"
  })
}

###########################################
# Primary Database Instance
###########################################

resource "aws_db_instance" "primary" {
  count = var.is_primary ? 1 : 0
  
  identifier = "${var.project_name}-${local.region_code}-primary"
  
  # Engine configuration
  engine         = "postgres"
  engine_version = "15.4"
  instance_class = var.db_instance_class
  
  # Storage configuration
  allocated_storage     = var.db_allocated_storage
  max_allocated_storage = var.db_allocated_storage * 2
  storage_type          = "gp3"
  storage_encrypted     = true
  kms_key_id           = aws_kms_key.rds[0].arn
  
  # Database configuration
  db_name  = replace(var.project_name, "-", "_")
  username = "hotelreviews_admin"
  password = random_password.db_password.result
  port     = 5432
  
  # Network configuration
  vpc_security_group_ids = [aws_security_group.rds.id]
  db_subnet_group_name   = aws_db_subnet_group.main.name
  publicly_accessible    = false
  
  # High Availability and Backup
  multi_az               = true
  backup_retention_period = 30
  backup_window          = "03:00-04:00"  # UTC
  maintenance_window     = "sun:04:00-sun:05:00"  # UTC
  delete_automated_backups = false
  deletion_protection    = true
  
  # Performance and Monitoring
  parameter_group_name        = aws_db_parameter_group.main.name
  performance_insights_enabled = true
  performance_insights_retention_period = 7
  monitoring_interval         = 60
  monitoring_role_arn        = aws_iam_role.rds_monitoring.arn
  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]
  
  # Snapshot configuration
  skip_final_snapshot       = false
  final_snapshot_identifier = "${var.project_name}-${local.region_code}-final-snapshot-${formatdate("YYYY-MM-DD-hhmm", timestamp())}"
  copy_tags_to_snapshot     = true
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-primary-db"
    Type = "Primary"
  })
  
  lifecycle {
    prevent_destroy = true
    ignore_changes = [
      password,  # Managed outside Terraform after initial creation
      final_snapshot_identifier
    ]
  }
}

###########################################
# Read Replica Instances
###########################################

resource "aws_db_instance" "replica" {
  count = var.is_primary ? 0 : 1
  
  identifier = "${var.project_name}-${local.region_code}-replica"
  
  # Replica configuration
  replicate_source_db = var.primary_db_identifier
  instance_class      = var.db_instance_class
  
  # Storage configuration (inherited from primary)
  storage_encrypted = true
  kms_key_id       = var.primary_db_kms_key_id
  
  # Network configuration
  vpc_security_group_ids = [aws_security_group.rds.id]
  db_subnet_group_name   = aws_db_subnet_group.main.name
  publicly_accessible    = false
  
  # Performance and Monitoring
  parameter_group_name        = aws_db_parameter_group.main.name
  performance_insights_enabled = true
  performance_insights_retention_period = 7
  monitoring_interval         = 60
  monitoring_role_arn        = aws_iam_role.rds_monitoring.arn
  
  # Read replica specific settings
  auto_minor_version_upgrade = true
  backup_retention_period    = 7  # Shorter retention for replicas
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-replica-db"
    Type = "ReadReplica"
  })
  
  lifecycle {
    prevent_destroy = true
  }
}

###########################################
# Database Password
###########################################

resource "random_password" "db_password" {
  length  = 32
  special = true
}

# Store password in AWS Secrets Manager
resource "aws_secretsmanager_secret" "db_password" {
  name        = "${var.project_name}-${local.region_code}-db-password"
  description = "Database password for ${var.project_name} in ${local.region_name}"
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-db-password"
  })
}

resource "aws_secretsmanager_secret_version" "db_password" {
  secret_id = aws_secretsmanager_secret.db_password.id
  secret_string = jsonencode({
    username = var.is_primary ? aws_db_instance.primary[0].username : "readonly_user"
    password = random_password.db_password.result
    engine   = "postgres"
    host     = var.is_primary ? aws_db_instance.primary[0].endpoint : aws_db_instance.replica[0].endpoint
    port     = 5432
    dbname   = var.is_primary ? aws_db_instance.primary[0].db_name : "hotel_reviews"
  })
}

###########################################
# IAM Role for RDS Monitoring
###########################################

resource "aws_iam_role" "rds_monitoring" {
  name = "${var.project_name}-${local.region_code}-rds-monitoring"
  
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "monitoring.rds.amazonaws.com"
        }
      }
    ]
  })
  
  tags = local.common_tags
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  role       = aws_iam_role.rds_monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

###########################################
# Database Subnet Group for Cache
###########################################

resource "aws_elasticache_subnet_group" "main" {
  name       = "${var.project_name}-${local.region_code}-cache-subnet-group"
  subnet_ids = aws_subnet.private[*].id
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-cache-subnet-group"
  })
}

###########################################
# Redis Cache Cluster
###########################################

resource "aws_elasticache_parameter_group" "redis" {
  family = "redis7.x"
  name   = "${var.project_name}-${local.region_code}-redis7"
  
  parameter {
    name  = "maxmemory-policy"
    value = "allkeys-lru"
  }
  
  parameter {
    name  = "timeout"
    value = "300"
  }
  
  tags = local.common_tags
}

resource "aws_elasticache_replication_group" "main" {
  replication_group_id       = "${var.project_name}-${local.region_code}-redis"
  description                = "Redis cluster for ${var.project_name} in ${local.region_name}"
  
  # Engine configuration
  engine               = "redis"
  engine_version       = "7.0"
  node_type           = var.redis_node_type
  parameter_group_name = aws_elasticache_parameter_group.redis.name
  port                = 6379
  
  # Cluster configuration
  num_cache_clusters = var.redis_num_cache_nodes
  
  # Network configuration
  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [aws_security_group.redis.id]
  
  # Security and Backup
  at_rest_encryption_enabled = true
  transit_encryption_enabled = true
  auth_token                 = random_password.redis_auth_token.result
  
  # Backup configuration
  snapshot_retention_limit = 5
  snapshot_window         = "05:00-06:00"  # UTC
  maintenance_window      = "sun:06:00-sun:07:00"  # UTC
  
  # Automatic failover (only for multi-node clusters)
  automatic_failover_enabled = var.redis_num_cache_nodes > 1
  multi_az_enabled          = var.redis_num_cache_nodes > 1
  
  # Logging
  log_delivery_configuration {
    destination      = aws_cloudwatch_log_group.redis_slow.name
    destination_type = "cloudwatch-logs"
    log_format       = "text"
    log_type         = "slow-log"
  }
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-redis"
  })
}

# Redis auth token
resource "random_password" "redis_auth_token" {
  length  = 32
  special = false  # Redis auth tokens don't support all special characters
}

# Store Redis auth token in Secrets Manager
resource "aws_secretsmanager_secret" "redis_auth_token" {
  name        = "${var.project_name}-${local.region_code}-redis-auth"
  description = "Redis auth token for ${var.project_name} in ${local.region_name}"
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-redis-auth"
  })
}

resource "aws_secretsmanager_secret_version" "redis_auth_token" {
  secret_id = aws_secretsmanager_secret.redis_auth_token.id
  secret_string = jsonencode({
    auth_token = random_password.redis_auth_token.result
    endpoint   = aws_elasticache_replication_group.main.configuration_endpoint_address
    port       = 6379
  })
}

# CloudWatch Log Group for Redis slow log
resource "aws_cloudwatch_log_group" "redis_slow" {
  name              = "/aws/elasticache/${var.project_name}-${local.region_code}-redis/slow-log"
  retention_in_days = 14
  
  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-redis-slow-log"
  })
}