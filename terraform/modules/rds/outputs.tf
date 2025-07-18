output "db_instance_id" {
  description = "RDS instance ID"
  value       = aws_db_instance.main.id
}

output "db_instance_address" {
  description = "RDS instance address"
  value       = aws_db_instance.main.address
}

output "db_instance_arn" {
  description = "RDS instance ARN"
  value       = aws_db_instance.main.arn
}

output "db_instance_endpoint" {
  description = "RDS instance endpoint"
  value       = aws_db_instance.main.endpoint
}

output "db_instance_port" {
  description = "RDS instance port"
  value       = aws_db_instance.main.port
}

output "db_instance_name" {
  description = "RDS instance database name"
  value       = aws_db_instance.main.db_name
}

output "db_instance_username" {
  description = "RDS instance username"
  value       = aws_db_instance.main.username
  sensitive   = true
}

output "db_subnet_group_id" {
  description = "DB subnet group ID"
  value       = aws_db_subnet_group.main.id
}

output "db_subnet_group_arn" {
  description = "DB subnet group ARN"
  value       = aws_db_subnet_group.main.arn
}

output "db_parameter_group_id" {
  description = "DB parameter group ID"
  value       = aws_db_parameter_group.main.id
}

output "db_parameter_group_arn" {
  description = "DB parameter group ARN"
  value       = aws_db_parameter_group.main.arn
}

output "db_option_group_id" {
  description = "DB option group ID"
  value       = aws_db_option_group.main.id
}

output "db_option_group_arn" {
  description = "DB option group ARN"
  value       = aws_db_option_group.main.arn
}

output "db_security_group_id" {
  description = "Database security group ID"
  value       = aws_security_group.rds.id
}

output "read_replica_id" {
  description = "Read replica ID"
  value       = var.create_read_replica ? aws_db_instance.read_replica[0].id : null
}

output "read_replica_address" {
  description = "Read replica address"
  value       = var.create_read_replica ? aws_db_instance.read_replica[0].address : null
}

output "read_replica_endpoint" {
  description = "Read replica endpoint"
  value       = var.create_read_replica ? aws_db_instance.read_replica[0].endpoint : null
}

output "enhanced_monitoring_role_arn" {
  description = "Enhanced monitoring role ARN"
  value       = var.monitoring_interval > 0 ? aws_iam_role.rds_enhanced_monitoring[0].arn : null
}

output "db_endpoint_ssm_parameter" {
  description = "SSM parameter name for database endpoint"
  value       = aws_ssm_parameter.db_endpoint.name
}

output "db_port_ssm_parameter" {
  description = "SSM parameter name for database port"
  value       = aws_ssm_parameter.db_port.name
}

output "db_name_ssm_parameter" {
  description = "SSM parameter name for database name"
  value       = aws_ssm_parameter.db_name.name
}

output "db_username_ssm_parameter" {
  description = "SSM parameter name for database username"
  value       = aws_ssm_parameter.db_username.name
}

output "db_password_ssm_parameter" {
  description = "SSM parameter name for database password"
  value       = aws_ssm_parameter.db_password.name
}