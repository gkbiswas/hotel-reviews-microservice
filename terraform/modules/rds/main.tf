# RDS Subnet Group
resource "aws_db_subnet_group" "main" {
  name       = "${var.db_name}-subnet-group"
  subnet_ids = var.private_subnet_ids

  tags = merge(var.tags, {
    Name = "${var.db_name} DB subnet group"
  })
}

# RDS Parameter Group
resource "aws_db_parameter_group" "main" {
  family = "postgres15"
  name   = "${var.db_name}-parameter-group"

  parameter {
    name  = "log_statement"
    value = "all"
  }

  parameter {
    name  = "log_min_duration_statement"
    value = "1000"
  }

  parameter {
    name  = "log_checkpoints"
    value = "on"
  }

  parameter {
    name  = "log_connections"
    value = "on"
  }

  parameter {
    name  = "log_disconnections"
    value = "on"
  }

  parameter {
    name  = "log_lock_waits"
    value = "on"
  }

  parameter {
    name  = "log_temp_files"
    value = "0"
  }

  parameter {
    name  = "log_autovacuum_min_duration"
    value = "0"
  }

  parameter {
    name  = "shared_preload_libraries"
    value = "pg_stat_statements"
  }

  tags = var.tags
}

# RDS Option Group
resource "aws_db_option_group" "main" {
  name                     = "${var.db_name}-option-group"
  option_group_description = "Option group for ${var.db_name}"
  engine_name              = "postgres"
  major_engine_version     = "15"

  tags = var.tags
}

# Security Group for RDS
resource "aws_security_group" "rds" {
  name        = "${var.db_name}-rds-sg"
  description = "Security group for RDS database"
  vpc_id      = var.vpc_id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = var.allowed_security_groups
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    Name = "${var.db_name}-rds-sg"
  })
}

# RDS Instance
resource "aws_db_instance" "main" {
  identifier = var.db_name

  # Engine configuration
  engine         = "postgres"
  engine_version = "15.4"
  instance_class = var.instance_class

  # Database configuration
  allocated_storage     = var.allocated_storage
  max_allocated_storage = var.max_allocated_storage
  storage_type          = "gp3"
  storage_encrypted     = true
  kms_key_id           = var.kms_key_id

  # Database credentials
  db_name  = var.database_name
  username = var.username
  password = var.password

  # Network configuration
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false

  # Parameter and option groups
  parameter_group_name = aws_db_parameter_group.main.name
  option_group_name    = aws_db_option_group.main.name

  # Backup configuration
  backup_retention_period = var.backup_retention_period
  backup_window          = var.backup_window
  maintenance_window     = var.maintenance_window

  # Monitoring and logging
  monitoring_interval = var.monitoring_interval
  monitoring_role_arn = var.monitoring_interval > 0 ? aws_iam_role.rds_enhanced_monitoring[0].arn : null

  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]

  # Performance insights
  performance_insights_enabled          = var.performance_insights_enabled
  performance_insights_retention_period = var.performance_insights_retention_period
  performance_insights_kms_key_id      = var.kms_key_id

  # High availability
  multi_az               = var.multi_az
  availability_zone      = var.multi_az ? null : var.availability_zone

  # Security
  deletion_protection = var.deletion_protection
  skip_final_snapshot = var.skip_final_snapshot
  final_snapshot_identifier = var.skip_final_snapshot ? null : "${var.db_name}-final-snapshot-${formatdate("YYYY-MM-DD-hhmm", timestamp())}"

  # Auto minor version upgrade
  auto_minor_version_upgrade = var.auto_minor_version_upgrade

  # Apply changes immediately
  apply_immediately = var.apply_immediately

  tags = merge(var.tags, {
    Name = var.db_name
  })

  depends_on = [
    aws_db_subnet_group.main,
    aws_db_parameter_group.main,
    aws_db_option_group.main
  ]
}

# Read Replica (optional)
resource "aws_db_instance" "read_replica" {
  count = var.create_read_replica ? 1 : 0

  identifier = "${var.db_name}-read-replica"

  # Engine configuration
  instance_class = var.read_replica_instance_class

  # Source database
  replicate_source_db = aws_db_instance.main.identifier

  # Network configuration
  publicly_accessible = false

  # Monitoring and logging
  monitoring_interval = var.monitoring_interval
  monitoring_role_arn = var.monitoring_interval > 0 ? aws_iam_role.rds_enhanced_monitoring[0].arn : null

  enabled_cloudwatch_logs_exports = ["postgresql", "upgrade"]

  # Performance insights
  performance_insights_enabled          = var.performance_insights_enabled
  performance_insights_retention_period = var.performance_insights_retention_period
  performance_insights_kms_key_id      = var.kms_key_id

  # High availability
  multi_az          = var.read_replica_multi_az
  availability_zone = var.read_replica_multi_az ? null : var.read_replica_availability_zone

  # Security
  deletion_protection = var.deletion_protection
  skip_final_snapshot = var.skip_final_snapshot

  # Auto minor version upgrade
  auto_minor_version_upgrade = var.auto_minor_version_upgrade

  # Apply changes immediately
  apply_immediately = var.apply_immediately

  tags = merge(var.tags, {
    Name = "${var.db_name}-read-replica"
  })

  depends_on = [aws_db_instance.main]
}

# IAM Role for RDS Enhanced Monitoring
resource "aws_iam_role" "rds_enhanced_monitoring" {
  count = var.monitoring_interval > 0 ? 1 : 0

  name = "${var.db_name}-rds-enhanced-monitoring-role"

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

  tags = var.tags
}

# IAM Policy Attachment for RDS Enhanced Monitoring
resource "aws_iam_role_policy_attachment" "rds_enhanced_monitoring" {
  count = var.monitoring_interval > 0 ? 1 : 0

  role       = aws_iam_role.rds_enhanced_monitoring[0].name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

# CloudWatch Alarms for RDS
resource "aws_cloudwatch_metric_alarm" "database_cpu" {
  alarm_name          = "${var.db_name}-database-cpu"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/RDS"
  period              = "120"
  statistic           = "Average"
  threshold           = "80"
  alarm_description   = "This metric monitors rds cpu utilization"
  alarm_actions       = var.alarm_actions

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.id
  }

  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "database_disk_queue" {
  alarm_name          = "${var.db_name}-database-disk-queue"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "DiskQueueDepth"
  namespace           = "AWS/RDS"
  period              = "120"
  statistic           = "Average"
  threshold           = "64"
  alarm_description   = "This metric monitors rds disk queue depth"
  alarm_actions       = var.alarm_actions

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.id
  }

  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "database_disk_free" {
  alarm_name          = "${var.db_name}-database-disk-free"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "FreeStorageSpace"
  namespace           = "AWS/RDS"
  period              = "120"
  statistic           = "Average"
  threshold           = "2000000000" # 2GB in bytes
  alarm_description   = "This metric monitors rds free storage space"
  alarm_actions       = var.alarm_actions

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.id
  }

  tags = var.tags
}

resource "aws_cloudwatch_metric_alarm" "database_memory_free" {
  alarm_name          = "${var.db_name}-database-memory-free"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "FreeableMemory"
  namespace           = "AWS/RDS"
  period              = "120"
  statistic           = "Average"
  threshold           = "128000000" # 128MB in bytes
  alarm_description   = "This metric monitors rds freeable memory"
  alarm_actions       = var.alarm_actions

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.id
  }

  tags = var.tags
}

# Database connection count alarm
resource "aws_cloudwatch_metric_alarm" "database_connections" {
  alarm_name          = "${var.db_name}-database-connections"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "DatabaseConnections"
  namespace           = "AWS/RDS"
  period              = "120"
  statistic           = "Average"
  threshold           = "80"
  alarm_description   = "This metric monitors rds database connections"
  alarm_actions       = var.alarm_actions

  dimensions = {
    DBInstanceIdentifier = aws_db_instance.main.id
  }

  tags = var.tags
}

# SSM Parameter for database endpoint
resource "aws_ssm_parameter" "db_endpoint" {
  name  = "/${var.environment}/${var.service_name}/database/endpoint"
  type  = "String"
  value = aws_db_instance.main.endpoint

  tags = var.tags
}

# SSM Parameter for database port
resource "aws_ssm_parameter" "db_port" {
  name  = "/${var.environment}/${var.service_name}/database/port"
  type  = "String"
  value = aws_db_instance.main.port

  tags = var.tags
}

# SSM Parameter for database name
resource "aws_ssm_parameter" "db_name" {
  name  = "/${var.environment}/${var.service_name}/database/name"
  type  = "String"
  value = aws_db_instance.main.db_name

  tags = var.tags
}

# SSM Parameter for database username
resource "aws_ssm_parameter" "db_username" {
  name  = "/${var.environment}/${var.service_name}/database/username"
  type  = "String"
  value = aws_db_instance.main.username

  tags = var.tags
}

# SSM Parameter for database password (SecureString)
resource "aws_ssm_parameter" "db_password" {
  name  = "/${var.environment}/${var.service_name}/database/password"
  type  = "SecureString"
  value = var.password

  tags = var.tags
}