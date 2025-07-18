# CloudWatch Alarms for Hotel Reviews Application
# This file contains comprehensive monitoring and alerting configuration

terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Variables
variable "environment" {
  description = "Environment name"
  type        = string
}

variable "project_name" {
  description = "Project name"
  type        = string
  default     = "hotel-reviews"
}

variable "service_name" {
  description = "Service name"
  type        = string
  default     = "hotel-reviews-service"
}

variable "cluster_name" {
  description = "ECS cluster name"
  type        = string
}

variable "blue_service_name" {
  description = "Blue ECS service name"
  type        = string
}

variable "green_service_name" {
  description = "Green ECS service name"
  type        = string
}

variable "load_balancer_arn" {
  description = "Application Load Balancer ARN"
  type        = string
}

variable "db_instance_id" {
  description = "RDS database instance ID"
  type        = string
}

variable "sns_topic_arn" {
  description = "SNS topic ARN for alerts"
  type        = string
}

# Extract load balancer name from ARN
locals {
  load_balancer_name = split("/", var.load_balancer_arn)[1]
}

# ECS Service Alarms
resource "aws_cloudwatch_metric_alarm" "ecs_cpu_high" {
  alarm_name          = "${var.project_name}-${var.environment}-ecs-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/ECS"
  period              = "300"
  statistic           = "Average"
  threshold           = "80"
  alarm_description   = "This metric monitors ECS CPU utilization"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    ServiceName = var.blue_service_name
    ClusterName = var.cluster_name
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "ecs_memory_high" {
  alarm_name          = "${var.project_name}-${var.environment}-ecs-memory-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "MemoryUtilization"
  namespace           = "AWS/ECS"
  period              = "300"
  statistic           = "Average"
  threshold           = "85"
  alarm_description   = "This metric monitors ECS memory utilization"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    ServiceName = var.blue_service_name
    ClusterName = var.cluster_name
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "ecs_service_running_count" {
  alarm_name          = "${var.project_name}-${var.environment}-ecs-running-count-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "RunningCount"
  namespace           = "AWS/ECS"
  period              = "60"
  statistic           = "Average"
  threshold           = "1"
  alarm_description   = "This metric monitors ECS running task count"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    ServiceName = var.blue_service_name
    ClusterName = var.cluster_name
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

# Application Load Balancer Alarms
resource "aws_cloudwatch_metric_alarm" "alb_response_time_high" {
  alarm_name          = "${var.project_name}-${var.environment}-alb-response-time-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "TargetResponseTime"
  namespace           = "AWS/ApplicationELB"
  period              = "300"
  statistic           = "Average"
  threshold           = "2"
  alarm_description   = "This metric monitors ALB response time"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    LoadBalancer = local.load_balancer_name
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "alb_5xx_errors_high" {
  alarm_name          = "${var.project_name}-${var.environment}-alb-5xx-errors-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "HTTPCode_Target_5XX_Count"
  namespace           = "AWS/ApplicationELB"
  period              = "300"
  statistic           = "Sum"
  threshold           = "10"
  alarm_description   = "This metric monitors ALB 5xx errors"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    LoadBalancer = local.load_balancer_name
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "alb_4xx_errors_high" {
  alarm_name          = "${var.project_name}-${var.environment}-alb-4xx-errors-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "3"
  metric_name         = "HTTPCode_Target_4XX_Count"
  namespace           = "AWS/ApplicationELB"
  period              = "300"
  statistic           = "Sum"
  threshold           = "50"
  alarm_description   = "This metric monitors ALB 4xx errors"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    LoadBalancer = local.load_balancer_name
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "alb_unhealthy_hosts" {
  alarm_name          = "${var.project_name}-${var.environment}-alb-unhealthy-hosts"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "UnHealthyHostCount"
  namespace           = "AWS/ApplicationELB"
  period              = "60"
  statistic           = "Average"
  threshold           = "0"
  alarm_description   = "This metric monitors ALB unhealthy hosts"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    LoadBalancer = local.load_balancer_name
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "alb_healthy_hosts_low" {
  alarm_name          = "${var.project_name}-${var.environment}-alb-healthy-hosts-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "HealthyHostCount"
  namespace           = "AWS/ApplicationELB"
  period              = "60"
  statistic           = "Average"
  threshold           = "1"
  alarm_description   = "This metric monitors ALB healthy hosts"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    LoadBalancer = local.load_balancer_name
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

# RDS Database Alarms
resource "aws_cloudwatch_metric_alarm" "rds_cpu_high" {
  alarm_name          = "${var.project_name}-${var.environment}-rds-cpu-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "CPUUtilization"
  namespace           = "AWS/RDS"
  period              = "300"
  statistic           = "Average"
  threshold           = "80"
  alarm_description   = "This metric monitors RDS CPU utilization"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    DBInstanceIdentifier = var.db_instance_id
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_disk_queue_depth_high" {
  alarm_name          = "${var.project_name}-${var.environment}-rds-disk-queue-depth-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "DiskQueueDepth"
  namespace           = "AWS/RDS"
  period              = "300"
  statistic           = "Average"
  threshold           = "10"
  alarm_description   = "This metric monitors RDS disk queue depth"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    DBInstanceIdentifier = var.db_instance_id
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_free_storage_space_low" {
  alarm_name          = "${var.project_name}-${var.environment}-rds-free-storage-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "FreeStorageSpace"
  namespace           = "AWS/RDS"
  period              = "300"
  statistic           = "Average"
  threshold           = "2000000000"  # 2GB in bytes
  alarm_description   = "This metric monitors RDS free storage space"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    DBInstanceIdentifier = var.db_instance_id
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_freeable_memory_low" {
  alarm_name          = "${var.project_name}-${var.environment}-rds-freeable-memory-low"
  comparison_operator = "LessThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "FreeableMemory"
  namespace           = "AWS/RDS"
  period              = "300"
  statistic           = "Average"
  threshold           = "128000000"  # 128MB in bytes
  alarm_description   = "This metric monitors RDS freeable memory"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    DBInstanceIdentifier = var.db_instance_id
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_connection_count_high" {
  alarm_name          = "${var.project_name}-${var.environment}-rds-connection-count-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "DatabaseConnections"
  namespace           = "AWS/RDS"
  period              = "300"
  statistic           = "Average"
  threshold           = "80"
  alarm_description   = "This metric monitors RDS database connections"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    DBInstanceIdentifier = var.db_instance_id
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_read_latency_high" {
  alarm_name          = "${var.project_name}-${var.environment}-rds-read-latency-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "ReadLatency"
  namespace           = "AWS/RDS"
  period              = "300"
  statistic           = "Average"
  threshold           = "0.2"
  alarm_description   = "This metric monitors RDS read latency"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    DBInstanceIdentifier = var.db_instance_id
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_metric_alarm" "rds_write_latency_high" {
  alarm_name          = "${var.project_name}-${var.environment}-rds-write-latency-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "WriteLatency"
  namespace           = "AWS/RDS"
  period              = "300"
  statistic           = "Average"
  threshold           = "0.2"
  alarm_description   = "This metric monitors RDS write latency"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]

  dimensions = {
    DBInstanceIdentifier = var.db_instance_id
  }

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

# Custom Application Metrics
resource "aws_cloudwatch_log_metric_filter" "application_errors" {
  name           = "${var.project_name}-${var.environment}-application-errors"
  log_group_name = "/ecs/${var.service_name}/app"
  pattern        = "[timestamp, request_id, level=\"ERROR\", ...]"

  metric_transformation {
    name      = "ApplicationErrors"
    namespace = "HotelReviews/Application"
    value     = "1"
  }
}

resource "aws_cloudwatch_metric_alarm" "application_errors_high" {
  alarm_name          = "${var.project_name}-${var.environment}-application-errors-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "ApplicationErrors"
  namespace           = "HotelReviews/Application"
  period              = "300"
  statistic           = "Sum"
  threshold           = "10"
  alarm_description   = "This metric monitors application errors"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]
  treat_missing_data  = "notBreaching"

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

# Log Metric Filter for Database Connection Errors
resource "aws_cloudwatch_log_metric_filter" "database_connection_errors" {
  name           = "${var.project_name}-${var.environment}-database-connection-errors"
  log_group_name = "/ecs/${var.service_name}/app"
  pattern        = "[timestamp, request_id, level, message=\"*database*connection*error*\", ...]"

  metric_transformation {
    name      = "DatabaseConnectionErrors"
    namespace = "HotelReviews/Application"
    value     = "1"
  }
}

resource "aws_cloudwatch_metric_alarm" "database_connection_errors_high" {
  alarm_name          = "${var.project_name}-${var.environment}-database-connection-errors-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "1"
  metric_name         = "DatabaseConnectionErrors"
  namespace           = "HotelReviews/Application"
  period              = "300"
  statistic           = "Sum"
  threshold           = "5"
  alarm_description   = "This metric monitors database connection errors"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]
  treat_missing_data  = "notBreaching"

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

# Log Metric Filter for Slow Queries
resource "aws_cloudwatch_log_metric_filter" "slow_queries" {
  name           = "${var.project_name}-${var.environment}-slow-queries"
  log_group_name = "/ecs/${var.service_name}/app"
  pattern        = "[timestamp, request_id, level, message=\"*slow*query*\", duration_ms > 5000, ...]"

  metric_transformation {
    name      = "SlowQueries"
    namespace = "HotelReviews/Application"
    value     = "1"
  }
}

resource "aws_cloudwatch_metric_alarm" "slow_queries_high" {
  alarm_name          = "${var.project_name}-${var.environment}-slow-queries-high"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "SlowQueries"
  namespace           = "HotelReviews/Application"
  period              = "300"
  statistic           = "Sum"
  threshold           = "10"
  alarm_description   = "This metric monitors slow database queries"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]
  treat_missing_data  = "notBreaching"

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

# Composite Alarms for Service Health
resource "aws_cloudwatch_composite_alarm" "service_health_critical" {
  alarm_name          = "${var.project_name}-${var.environment}-service-health-critical"
  alarm_description   = "Composite alarm for critical service health issues"
  alarm_rule          = "ALARM(${aws_cloudwatch_metric_alarm.ecs_service_running_count.alarm_name}) OR ALARM(${aws_cloudwatch_metric_alarm.alb_healthy_hosts_low.alarm_name}) OR ALARM(${aws_cloudwatch_metric_alarm.rds_connection_count_high.alarm_name})"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]
  
  actions_enabled = true

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

resource "aws_cloudwatch_composite_alarm" "service_health_warning" {
  alarm_name          = "${var.project_name}-${var.environment}-service-health-warning"
  alarm_description   = "Composite alarm for service health warnings"
  alarm_rule          = "ALARM(${aws_cloudwatch_metric_alarm.ecs_cpu_high.alarm_name}) OR ALARM(${aws_cloudwatch_metric_alarm.ecs_memory_high.alarm_name}) OR ALARM(${aws_cloudwatch_metric_alarm.alb_response_time_high.alarm_name}) OR ALARM(${aws_cloudwatch_metric_alarm.rds_cpu_high.alarm_name})"
  alarm_actions       = [var.sns_topic_arn]
  ok_actions          = [var.sns_topic_arn]
  
  actions_enabled = true

  tags = {
    Environment = var.environment
    Project     = var.project_name
    Service     = var.service_name
  }
}

# Outputs
output "alarm_names" {
  description = "List of created CloudWatch alarm names"
  value = [
    aws_cloudwatch_metric_alarm.ecs_cpu_high.alarm_name,
    aws_cloudwatch_metric_alarm.ecs_memory_high.alarm_name,
    aws_cloudwatch_metric_alarm.ecs_service_running_count.alarm_name,
    aws_cloudwatch_metric_alarm.alb_response_time_high.alarm_name,
    aws_cloudwatch_metric_alarm.alb_5xx_errors_high.alarm_name,
    aws_cloudwatch_metric_alarm.alb_4xx_errors_high.alarm_name,
    aws_cloudwatch_metric_alarm.alb_unhealthy_hosts.alarm_name,
    aws_cloudwatch_metric_alarm.alb_healthy_hosts_low.alarm_name,
    aws_cloudwatch_metric_alarm.rds_cpu_high.alarm_name,
    aws_cloudwatch_metric_alarm.rds_disk_queue_depth_high.alarm_name,
    aws_cloudwatch_metric_alarm.rds_free_storage_space_low.alarm_name,
    aws_cloudwatch_metric_alarm.rds_freeable_memory_low.alarm_name,
    aws_cloudwatch_metric_alarm.rds_connection_count_high.alarm_name,
    aws_cloudwatch_metric_alarm.rds_read_latency_high.alarm_name,
    aws_cloudwatch_metric_alarm.rds_write_latency_high.alarm_name,
    aws_cloudwatch_metric_alarm.application_errors_high.alarm_name,
    aws_cloudwatch_metric_alarm.database_connection_errors_high.alarm_name,
    aws_cloudwatch_metric_alarm.slow_queries_high.alarm_name,
    aws_cloudwatch_composite_alarm.service_health_critical.alarm_name,
    aws_cloudwatch_composite_alarm.service_health_warning.alarm_name,
  ]
}

output "metric_filter_names" {
  description = "List of created CloudWatch log metric filter names"
  value = [
    aws_cloudwatch_log_metric_filter.application_errors.name,
    aws_cloudwatch_log_metric_filter.database_connection_errors.name,
    aws_cloudwatch_log_metric_filter.slow_queries.name,
  ]
}