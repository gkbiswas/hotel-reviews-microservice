variable "db_name" {
  description = "Name of the database"
  type        = string
}

variable "database_name" {
  description = "Name of the database to create"
  type        = string
}

variable "username" {
  description = "Username for the database"
  type        = string
}

variable "password" {
  description = "Password for the database"
  type        = string
  sensitive   = true
}

variable "instance_class" {
  description = "Instance class for the database"
  type        = string
  default     = "db.t3.micro"
}

variable "allocated_storage" {
  description = "Allocated storage for the database"
  type        = number
  default     = 20
}

variable "max_allocated_storage" {
  description = "Maximum allocated storage for the database"
  type        = number
  default     = 100
}

variable "vpc_id" {
  description = "VPC ID"
  type        = string
}

variable "private_subnet_ids" {
  description = "List of private subnet IDs"
  type        = list(string)
}

variable "allowed_security_groups" {
  description = "List of security groups allowed to access the database"
  type        = list(string)
}

variable "backup_retention_period" {
  description = "Backup retention period in days"
  type        = number
  default     = 7
}

variable "backup_window" {
  description = "Backup window"
  type        = string
  default     = "07:00-09:00"
}

variable "maintenance_window" {
  description = "Maintenance window"
  type        = string
  default     = "sun:05:00-sun:06:00"
}

variable "monitoring_interval" {
  description = "Monitoring interval"
  type        = number
  default     = 60
}

variable "performance_insights_enabled" {
  description = "Enable performance insights"
  type        = bool
  default     = true
}

variable "performance_insights_retention_period" {
  description = "Performance insights retention period"
  type        = number
  default     = 7
}

variable "multi_az" {
  description = "Enable multi-AZ deployment"
  type        = bool
  default     = false
}

variable "availability_zone" {
  description = "Availability zone for single-AZ deployment"
  type        = string
  default     = null
}

variable "deletion_protection" {
  description = "Enable deletion protection"
  type        = bool
  default     = true
}

variable "skip_final_snapshot" {
  description = "Skip final snapshot"
  type        = bool
  default     = false
}

variable "auto_minor_version_upgrade" {
  description = "Enable auto minor version upgrade"
  type        = bool
  default     = true
}

variable "apply_immediately" {
  description = "Apply changes immediately"
  type        = bool
  default     = false
}

variable "create_read_replica" {
  description = "Create read replica"
  type        = bool
  default     = false
}

variable "read_replica_instance_class" {
  description = "Instance class for read replica"
  type        = string
  default     = "db.t3.micro"
}

variable "read_replica_multi_az" {
  description = "Enable multi-AZ for read replica"
  type        = bool
  default     = false
}

variable "read_replica_availability_zone" {
  description = "Availability zone for read replica"
  type        = string
  default     = null
}

variable "environment" {
  description = "Environment name"
  type        = string
}

variable "service_name" {
  description = "Service name"
  type        = string
}

variable "kms_key_id" {
  description = "KMS key ID for encryption"
  type        = string
  default     = null
}

variable "alarm_actions" {
  description = "List of alarm actions"
  type        = list(string)
  default     = []
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default     = {}
}