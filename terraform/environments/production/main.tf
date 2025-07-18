terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }

  backend "s3" {
    bucket         = "hotel-reviews-terraform-state-prod"
    key            = "production/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "hotel-reviews-terraform-locks"
  }
}

provider "aws" {
  region = var.aws_region
  
  default_tags {
    tags = {
      Environment = var.environment
      Project     = var.project_name
      ManagedBy   = "Terraform"
    }
  }
}

# Data sources for existing VPC and subnets
data "aws_vpc" "main" {
  filter {
    name   = "tag:Name"
    values = ["${var.project_name}-${var.environment}-vpc"]
  }
}

data "aws_subnets" "private" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.main.id]
  }
  filter {
    name   = "tag:Type"
    values = ["private"]
  }
}

data "aws_subnets" "public" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.main.id]
  }
  filter {
    name   = "tag:Type"
    values = ["public"]
  }
}

# S3 bucket for application data
resource "aws_s3_bucket" "app_data" {
  bucket = "${var.project_name}-${var.environment}-app-data"
}

resource "aws_s3_bucket_versioning" "app_data" {
  bucket = aws_s3_bucket.app_data.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_encryption" "app_data" {
  bucket = aws_s3_bucket.app_data.id

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}

resource "aws_s3_bucket_public_access_block" "app_data" {
  bucket = aws_s3_bucket.app_data.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# SNS topic for alerts
resource "aws_sns_topic" "alerts" {
  name = "${var.project_name}-${var.environment}-alerts"
}

resource "aws_sns_topic_subscription" "alerts_email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

# RDS Module
module "rds" {
  source = "../../modules/rds"

  db_name                    = "${var.project_name}-${var.environment}-db"
  database_name              = var.database_name
  username                   = var.database_username
  password                   = var.database_password
  instance_class             = var.rds_instance_class
  allocated_storage          = var.rds_allocated_storage
  max_allocated_storage      = var.rds_max_allocated_storage
  vpc_id                     = data.aws_vpc.main.id
  private_subnet_ids         = data.aws_subnets.private.ids
  allowed_security_groups    = []
  backup_retention_period    = var.rds_backup_retention_period
  backup_window             = var.rds_backup_window
  maintenance_window        = var.rds_maintenance_window
  monitoring_interval       = var.rds_monitoring_interval
  performance_insights_enabled = var.rds_performance_insights_enabled
  multi_az                  = var.rds_multi_az
  deletion_protection       = var.rds_deletion_protection
  skip_final_snapshot       = var.rds_skip_final_snapshot
  create_read_replica       = var.rds_create_read_replica
  read_replica_instance_class = var.rds_read_replica_instance_class
  environment               = var.environment
  service_name              = var.service_name
  alarm_actions             = [aws_sns_topic.alerts.arn]

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

# ECS Module
module "ecs" {
  source = "../../modules/ecs"

  cluster_name        = "${var.project_name}-${var.environment}-cluster"
  service_name        = var.service_name
  container_name      = var.container_name
  image_uri           = var.image_uri
  container_port      = var.container_port
  task_cpu            = var.task_cpu
  task_memory         = var.task_memory
  desired_count       = var.desired_count
  vpc_id              = data.aws_vpc.main.id
  private_subnet_ids  = data.aws_subnets.private.ids
  public_subnet_ids   = data.aws_subnets.public.ids
  environment         = var.environment
  aws_region          = var.aws_region
  s3_bucket_arn       = aws_s3_bucket.app_data.arn
  log_retention_days  = var.log_retention_days
  enable_autoscaling  = var.enable_autoscaling
  min_capacity        = var.min_capacity
  max_capacity        = var.max_capacity
  cpu_target_value    = var.cpu_target_value
  memory_target_value = var.memory_target_value

  environment_variables = {
    ENVIRONMENT = var.environment
    LOG_LEVEL   = var.log_level
    LOG_FORMAT  = var.log_format
  }

  secrets = {
    DATABASE_HOST     = module.rds.db_endpoint_ssm_parameter
    DATABASE_PORT     = module.rds.db_port_ssm_parameter
    DATABASE_NAME     = module.rds.db_name_ssm_parameter
    DATABASE_USER     = module.rds.db_username_ssm_parameter
    DATABASE_PASSWORD = module.rds.db_password_ssm_parameter
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }

  depends_on = [module.rds]
}

# Update RDS security group to allow ECS access
resource "aws_security_group_rule" "rds_ingress_ecs" {
  type                     = "ingress"
  from_port                = 5432
  to_port                  = 5432
  protocol                 = "tcp"
  security_group_id        = module.rds.db_security_group_id
  source_security_group_id = module.ecs.security_group_id
}

# CloudWatch Dashboard
resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = "${var.project_name}-${var.environment}-dashboard"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6

        properties = {
          metrics = [
            ["AWS/ECS", "CPUUtilization", "ServiceName", module.ecs.blue_service_name, "ClusterName", module.ecs.cluster_name],
            [".", "MemoryUtilization", ".", ".", ".", "."],
            ["AWS/ApplicationELB", "RequestCount", "LoadBalancer", module.ecs.load_balancer_arn],
            [".", "TargetResponseTime", ".", "."],
            ["AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", module.rds.db_instance_id],
            [".", "DatabaseConnections", ".", "."],
            [".", "ReadLatency", ".", "."],
            [".", "WriteLatency", ".", "."]
          ]
          period = 300
          stat   = "Average"
          region = var.aws_region
          title  = "Application Metrics"
        }
      }
    ]
  })
}

# Route 53 Health Check
resource "aws_route53_health_check" "main" {
  fqdn                            = module.ecs.load_balancer_dns
  port                            = 80
  type                            = "HTTP"
  resource_path                   = "/api/v1/health"
  failure_threshold               = "3"
  request_interval                = "30"
  cloudwatch_alarm_region         = var.aws_region
  cloudwatch_alarm_name           = "${var.project_name}-${var.environment}-health-check"
  insufficient_data_health_status = "Failure"

  tags = {
    Name        = "${var.project_name}-${var.environment}-health-check"
    Environment = var.environment
    Project     = var.project_name
  }
}

# CloudWatch alarm for health check
resource "aws_cloudwatch_metric_alarm" "health_check" {
  alarm_name          = "${var.project_name}-${var.environment}-health-check"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "HealthCheckStatus"
  namespace           = "AWS/Route53"
  period              = "60"
  statistic           = "Minimum"
  threshold           = "1"
  alarm_description   = "This metric monitors application health"
  alarm_actions       = [aws_sns_topic.alerts.arn]

  dimensions = {
    HealthCheckId = aws_route53_health_check.main.id
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
  }
}

# WAF for additional security
resource "aws_wafv2_web_acl" "main" {
  name  = "${var.project_name}-${var.environment}-waf"
  scope = "REGIONAL"

  default_action {
    allow {}
  }

  rule {
    name     = "RateLimitRule"
    priority = 1

    override_action {
      none {}
    }

    statement {
      rate_based_statement {
        limit              = 2000
        aggregate_key_type = "IP"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "RateLimitRule"
      sampled_requests_enabled   = true
    }

    action {
      block {}
    }
  }

  rule {
    name     = "AWSManagedRulesCommonRuleSet"
    priority = 2

    override_action {
      none {}
    }

    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"
      }
    }

    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "CommonRuleSetMetric"
      sampled_requests_enabled   = true
    }
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.project_name}-${var.environment}-waf"
    sampled_requests_enabled   = true
  }
}

# Associate WAF with ALB
resource "aws_wafv2_web_acl_association" "main" {
  resource_arn = module.ecs.load_balancer_arn
  web_acl_arn  = aws_wafv2_web_acl.main.arn
}

# Systems Manager parameters for deployment configuration
resource "aws_ssm_parameter" "deployment_config" {
  for_each = {
    blue_service_name       = module.ecs.blue_service_name
    green_service_name      = module.ecs.green_service_name
    blue_target_group_arn   = module.ecs.blue_target_group_arn
    green_target_group_arn  = module.ecs.green_target_group_arn
    load_balancer_arn       = module.ecs.load_balancer_arn
    listener_arn           = module.ecs.listener_arn
    listener_rule_arn      = module.ecs.listener_rule_arn
    cluster_name           = module.ecs.cluster_name
    task_definition_arn    = module.ecs.task_definition_arn
  }

  name  = "/${var.environment}/${var.service_name}/deployment/${each.key}"
  type  = "String"
  value = each.value

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}