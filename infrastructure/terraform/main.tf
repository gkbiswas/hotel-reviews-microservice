# Multi-Region Hotel Reviews Infrastructure
# Terraform configuration for global deployment

terraform {
  required_version = ">= 1.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  
  backend "s3" {
    bucket         = "hotel-reviews-terraform-state"
    key            = "multi-region/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-locks"
  }
}

# Global configuration
locals {
  project_name = "hotel-reviews"
  environment  = var.environment
  
  regions = {
    primary = {
      name = "us-east-1"
      code = "use1"
      vpc_cidr = "10.0.0.0/16"
    }
    secondary = {
      name = "eu-west-1" 
      code = "euw1"
      vpc_cidr = "10.1.0.0/16"
    }
    tertiary = {
      name = "ap-southeast-1"
      code = "apse1" 
      vpc_cidr = "10.2.0.0/16"
    }
  }
  
  common_tags = {
    Project     = local.project_name
    Environment = local.environment
    ManagedBy   = "terraform"
    Region      = "multi-region"
  }
}

# Primary region provider
provider "aws" {
  alias  = "primary"
  region = local.regions.primary.name
  
  default_tags {
    tags = merge(local.common_tags, {
      Region = local.regions.primary.name
      Type   = "primary"
    })
  }
}

# Secondary region provider  
provider "aws" {
  alias  = "secondary"
  region = local.regions.secondary.name
  
  default_tags {
    tags = merge(local.common_tags, {
      Region = local.regions.secondary.name
      Type   = "secondary"
    })
  }
}

# Tertiary region provider
provider "aws" {
  alias  = "tertiary"
  region = local.regions.tertiary.name
  
  default_tags {
    tags = merge(local.common_tags, {
      Region = local.regions.tertiary.name  
      Type   = "tertiary"
    })
  }
}

# Global Accelerator for traffic distribution
resource "aws_globalaccelerator_accelerator" "main" {
  provider = aws.primary
  
  name            = "${local.project_name}-global-accelerator"
  ip_address_type = "IPV4"
  enabled         = true
  
  attributes {
    flow_logs_enabled   = true
    flow_logs_s3_bucket = aws_s3_bucket.global_logs.bucket
    flow_logs_s3_prefix = "flow-logs/"
  }
  
  tags = merge(local.common_tags, {
    Name = "${local.project_name}-global-accelerator"
  })
}

# Global Accelerator Listener
resource "aws_globalaccelerator_listener" "main" {
  provider = aws.primary
  
  accelerator_arn = aws_globalaccelerator_accelerator.main.id
  client_affinity = "SOURCE_IP"
  protocol        = "TCP"
  
  port_range {
    from = 80
    to   = 80
  }
  
  port_range {
    from = 443
    to   = 443
  }
}

# Primary region infrastructure
module "primary_region" {
  source = "./modules/region"
  
  providers = {
    aws = aws.primary
  }
  
  project_name = local.project_name
  environment  = local.environment
  region_config = local.regions.primary
  is_primary   = true
  
  # Database configuration
  db_instance_class = var.primary_db_instance_class
  db_allocated_storage = var.primary_db_allocated_storage
  
  # EKS configuration
  eks_node_groups = var.primary_eks_node_groups
  
  # Cache configuration
  redis_node_type = var.primary_redis_node_type
  redis_num_cache_nodes = var.primary_redis_num_cache_nodes
  
  tags = local.common_tags
}

# Secondary region infrastructure
module "secondary_region" {
  source = "./modules/region"
  
  providers = {
    aws = aws.secondary
  }
  
  project_name = local.project_name
  environment  = local.environment  
  region_config = local.regions.secondary
  is_primary   = false
  
  # Read replica configuration
  primary_db_identifier = module.primary_region.db_identifier
  primary_db_kms_key_id = module.primary_region.db_kms_key_id
  
  # Smaller instance classes for secondary regions
  db_instance_class = var.secondary_db_instance_class
  eks_node_groups = var.secondary_eks_node_groups
  redis_node_type = var.secondary_redis_node_type
  redis_num_cache_nodes = var.secondary_redis_num_cache_nodes
  
  tags = local.common_tags
}

# Tertiary region infrastructure
module "tertiary_region" {
  source = "./modules/region"
  
  providers = {
    aws = aws.tertiary
  }
  
  project_name = local.project_name
  environment  = local.environment
  region_config = local.regions.tertiary
  is_primary   = false
  
  # Read replica configuration
  primary_db_identifier = module.primary_region.db_identifier
  primary_db_kms_key_id = module.primary_region.db_kms_key_id
  
  # Smallest instance classes for tertiary region
  db_instance_class = var.tertiary_db_instance_class
  eks_node_groups = var.tertiary_eks_node_groups
  redis_node_type = var.tertiary_redis_node_type
  redis_num_cache_nodes = var.tertiary_redis_num_cache_nodes
  
  tags = local.common_tags
}

# Global Accelerator Endpoint Groups
resource "aws_globalaccelerator_endpoint_group" "primary" {
  provider = aws.primary
  
  listener_arn = aws_globalaccelerator_listener.main.id
  
  endpoint_group_region = local.regions.primary.name
  traffic_dial_percentage = 100
  health_check_grace_period_seconds = 30
  health_check_interval_seconds = 30
  health_check_path = "/health"
  health_check_port = 80
  health_check_protocol = "HTTP"
  threshold_count = 3
  
  endpoint_configuration {
    endpoint_id = module.primary_region.alb_arn
    weight      = 100
  }
}

resource "aws_globalaccelerator_endpoint_group" "secondary" {
  provider = aws.secondary
  
  listener_arn = aws_globalaccelerator_listener.main.id
  
  endpoint_group_region = local.regions.secondary.name
  traffic_dial_percentage = 0  # Standby by default
  health_check_grace_period_seconds = 30
  health_check_interval_seconds = 30
  health_check_path = "/health"
  health_check_port = 80
  health_check_protocol = "HTTP"
  threshold_count = 3
  
  endpoint_configuration {
    endpoint_id = module.secondary_region.alb_arn
    weight      = 100
  }
}

resource "aws_globalaccelerator_endpoint_group" "tertiary" {
  provider = aws.tertiary
  
  listener_arn = aws_globalaccelerator_listener.main.id
  
  endpoint_group_region = local.regions.tertiary.name
  traffic_dial_percentage = 0  # Standby by default
  health_check_grace_period_seconds = 30
  health_check_interval_seconds = 30
  health_check_path = "/health"  
  health_check_port = 80
  health_check_protocol = "HTTP"
  threshold_count = 3
  
  endpoint_configuration {
    endpoint_id = module.tertiary_region.alb_arn
    weight      = 100
  }
}

# CloudFront Distribution for global content delivery
resource "aws_cloudfront_distribution" "main" {
  provider = aws.primary
  
  comment = "${local.project_name} Global CDN"
  enabled = true
  is_ipv6_enabled = true
  
  # Multiple origins for multi-region failover
  origin {
    domain_name = aws_globalaccelerator_accelerator.main.dns_name
    origin_id   = "GlobalAccelerator"
    
    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }
  
  # Static assets from S3
  origin {
    domain_name = aws_s3_bucket.static_assets.bucket_regional_domain_name
    origin_id   = "StaticAssets"
    
    s3_origin_config {
      origin_access_identity = aws_cloudfront_origin_access_identity.static.cloudfront_access_identity_path
    }
  }
  
  # Default cache behavior for API
  default_cache_behavior {
    allowed_methods        = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "GlobalAccelerator"
    compress              = true
    viewer_protocol_policy = "redirect-to-https"
    
    forwarded_values {
      query_string = true
      headers      = ["Authorization", "CloudFront-Forwarded-Proto"]
      
      cookies {
        forward = "none"
      }
    }
    
    min_ttl     = 0
    default_ttl = 300    # 5 minutes
    max_ttl     = 86400  # 24 hours
  }
  
  # Static assets cache behavior
  ordered_cache_behavior {
    path_pattern     = "/static/*"
    allowed_methods  = ["GET", "HEAD"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "StaticAssets"
    compress         = true
    viewer_protocol_policy = "redirect-to-https"
    
    forwarded_values {
      query_string = false
      cookies {
        forward = "none"
      }
    }
    
    min_ttl     = 86400   # 1 day
    default_ttl = 604800  # 1 week  
    max_ttl     = 31536000 # 1 year
  }
  
  # Geographic restrictions
  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }
  
  # SSL Certificate
  viewer_certificate {
    acm_certificate_arn = aws_acm_certificate.global.arn
    ssl_support_method  = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }
  
  # Custom error pages
  custom_error_response {
    error_code         = 404
    response_code      = 404
    response_page_path = "/error-pages/404.html"
  }
  
  custom_error_response {
    error_code         = 500
    response_code      = 500
    response_page_path = "/error-pages/500.html"
  }
  
  tags = merge(local.common_tags, {
    Name = "${local.project_name}-cloudfront"
  })
}

# Global S3 bucket for logs
resource "aws_s3_bucket" "global_logs" {
  provider = aws.primary
  
  bucket = "${local.project_name}-global-logs-${random_id.bucket_suffix.hex}"
  
  tags = merge(local.common_tags, {
    Name = "${local.project_name}-global-logs"
  })
}

# Static assets S3 bucket
resource "aws_s3_bucket" "static_assets" {
  provider = aws.primary
  
  bucket = "${local.project_name}-static-assets-${random_id.bucket_suffix.hex}"
  
  tags = merge(local.common_tags, {
    Name = "${local.project_name}-static-assets"
  })
}

# CloudFront Origin Access Identity
resource "aws_cloudfront_origin_access_identity" "static" {
  provider = aws.primary
  comment  = "OAI for ${local.project_name} static assets"
}

# ACM Certificate for global use
resource "aws_acm_certificate" "global" {
  provider = aws.primary  # Must be in us-east-1 for CloudFront
  
  domain_name       = var.domain_name
  subject_alternative_names = ["*.${var.domain_name}"]
  validation_method = "DNS"
  
  lifecycle {
    create_before_destroy = true
  }
  
  tags = merge(local.common_tags, {
    Name = "${local.project_name}-global-cert"
  })
}

# Random ID for unique bucket names
resource "random_id" "bucket_suffix" {
  byte_length = 4
}

# Route53 Health Checks for each region
resource "aws_route53_health_check" "primary" {
  provider = aws.primary
  
  fqdn                            = module.primary_region.alb_dns_name
  port                            = 443
  type                            = "HTTPS"
  resource_path                   = "/health"
  failure_threshold               = "3"
  request_interval                = "30"
  cloudwatch_logs_region          = local.regions.primary.name
  cloudwatch_alarm_region         = local.regions.primary.name
  insufficient_data_health_status = "Failure"
  
  tags = merge(local.common_tags, {
    Name = "${local.project_name}-primary-health-check"
  })
}

resource "aws_route53_health_check" "secondary" {
  provider = aws.primary
  
  fqdn                            = module.secondary_region.alb_dns_name
  port                            = 443
  type                            = "HTTPS"
  resource_path                   = "/health"
  failure_threshold               = "3"
  request_interval                = "30"
  cloudwatch_logs_region          = local.regions.secondary.name
  cloudwatch_alarm_region         = local.regions.secondary.name
  insufficient_data_health_status = "Failure"
  
  tags = merge(local.common_tags, {
    Name = "${local.project_name}-secondary-health-check"
  })
}

resource "aws_route53_health_check" "tertiary" {
  provider = aws.primary
  
  fqdn                            = module.tertiary_region.alb_dns_name
  port                            = 443
  type                            = "HTTPS"
  resource_path                   = "/health"
  failure_threshold               = "3"
  request_interval                = "30"
  cloudwatch_logs_region          = local.regions.tertiary.name
  cloudwatch_alarm_region         = local.regions.tertiary.name
  insufficient_data_health_status = "Failure"
  
  tags = merge(local.common_tags, {
    Name = "${local.project_name}-tertiary-health-check"
  })
}