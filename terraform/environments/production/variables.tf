variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "production"
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

variable "container_name" {
  description = "Container name"
  type        = string
  default     = "hotel-reviews-api"
}

variable "image_uri" {
  description = "Docker image URI"
  type        = string
  default     = "ghcr.io/gkbiswas/hotel-reviews-microservice:latest"
}

variable "container_port" {
  description = "Container port"
  type        = number
  default     = 8080
}

variable "task_cpu" {
  description = "Task CPU units"
  type        = number
  default     = 512
}

variable "task_memory" {
  description = "Task memory"
  type        = number
  default     = 1024
}

variable "desired_count" {
  description = "Desired number of tasks"
  type        = number
  default     = 3
}

variable "log_retention_days" {
  description = "Log retention period in days"
  type        = number
  default     = 30
}

variable "enable_autoscaling" {
  description = "Enable auto scaling"
  type        = bool
  default     = true
}

variable "min_capacity" {
  description = "Minimum capacity for auto scaling"
  type        = number
  default     = 2
}

variable "max_capacity" {
  description = "Maximum capacity for auto scaling"
  type        = number
  default     = 20
}

variable "cpu_target_value" {
  description = "Target CPU utilization for auto scaling"
  type        = number
  default     = 70
}

variable "memory_target_value" {
  description = "Target memory utilization for auto scaling"
  type        = number
  default     = 80
}

variable "log_level" {
  description = "Log level"
  type        = string
  default     = "info"
}

variable "log_format" {
  description = "Log format"
  type        = string
  default     = "json"
}

# Database variables
variable "database_name" {
  description = "Database name"
  type        = string
  default     = "hotel_reviews"
}

variable "database_username" {
  description = "Database username"
  type        = string
  default     = "hotel_reviews_user"
}

variable "database_password" {
  description = "Database password"
  type        = string
  sensitive   = true
}

variable "rds_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.medium"
}

variable "rds_allocated_storage" {
  description = "RDS allocated storage"
  type        = number
  default     = 100
}

variable "rds_max_allocated_storage" {
  description = "RDS max allocated storage"
  type        = number
  default     = 1000
}

variable "rds_backup_retention_period" {
  description = "RDS backup retention period"
  type        = number
  default     = 30
}

variable "rds_backup_window" {
  description = "RDS backup window"
  type        = string
  default     = "03:00-04:00"
}

variable "rds_maintenance_window" {
  description = "RDS maintenance window"
  type        = string
  default     = "sun:04:00-sun:05:00"
}

variable "rds_monitoring_interval" {
  description = "RDS monitoring interval"
  type        = number
  default     = 60
}

variable "rds_performance_insights_enabled" {
  description = "Enable RDS performance insights"
  type        = bool
  default     = true
}

variable "rds_multi_az" {
  description = "Enable RDS multi-AZ"
  type        = bool
  default     = true
}

variable "rds_deletion_protection" {
  description = "Enable RDS deletion protection"
  type        = bool
  default     = true
}

variable "rds_skip_final_snapshot" {
  description = "Skip RDS final snapshot"
  type        = bool
  default     = false
}

variable "rds_create_read_replica" {
  description = "Create RDS read replica"
  type        = bool
  default     = true
}

variable "rds_read_replica_instance_class" {
  description = "RDS read replica instance class"
  type        = string
  default     = "db.t3.medium"
}

variable "alert_email" {
  description = "Email address for alerts"
  type        = string
  default     = "ops@company.com"
}