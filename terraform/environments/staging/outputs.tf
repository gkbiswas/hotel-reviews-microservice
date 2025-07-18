output "load_balancer_dns" {
  description = "DNS name of the load balancer"
  value       = module.ecs.load_balancer_dns
}

output "load_balancer_arn" {
  description = "ARN of the load balancer"
  value       = module.ecs.load_balancer_arn
}

output "cluster_name" {
  description = "Name of the ECS cluster"
  value       = module.ecs.cluster_name
}

output "blue_service_name" {
  description = "Name of the blue ECS service"
  value       = module.ecs.blue_service_name
}

output "green_service_name" {
  description = "Name of the green ECS service"
  value       = module.ecs.green_service_name
}

output "blue_target_group_arn" {
  description = "ARN of the blue target group"
  value       = module.ecs.blue_target_group_arn
}

output "green_target_group_arn" {
  description = "ARN of the green target group"
  value       = module.ecs.green_target_group_arn
}

output "listener_arn" {
  description = "ARN of the load balancer listener"
  value       = module.ecs.listener_arn
}

output "listener_rule_arn" {
  description = "ARN of the load balancer listener rule"
  value       = module.ecs.listener_rule_arn
}

output "database_endpoint" {
  description = "RDS instance endpoint"
  value       = module.rds.db_instance_endpoint
}

output "database_port" {
  description = "RDS instance port"
  value       = module.rds.db_instance_port
}

output "s3_bucket_name" {
  description = "Name of the S3 bucket"
  value       = aws_s3_bucket.app_data.bucket
}

output "s3_bucket_arn" {
  description = "ARN of the S3 bucket"
  value       = aws_s3_bucket.app_data.arn
}

output "cloudwatch_dashboard_url" {
  description = "URL of the CloudWatch dashboard"
  value       = "https://console.aws.amazon.com/cloudwatch/home?region=${var.aws_region}#dashboards:name=${aws_cloudwatch_dashboard.main.dashboard_name}"
}

output "health_check_id" {
  description = "Route 53 health check ID"
  value       = aws_route53_health_check.main.id
}

output "sns_topic_arn" {
  description = "ARN of the SNS topic for alerts"
  value       = aws_sns_topic.alerts.arn
}