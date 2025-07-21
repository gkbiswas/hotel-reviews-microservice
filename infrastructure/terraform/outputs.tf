# Global Multi-Region Outputs
# Export important global infrastructure information

###########################################
# Global Accelerator Outputs
###########################################

output "global_accelerator_dns_name" {
  description = "DNS name of the Global Accelerator"
  value       = aws_globalaccelerator_accelerator.main.dns_name
}

output "global_accelerator_hosted_zone_id" {
  description = "Hosted zone ID of the Global Accelerator"
  value       = aws_globalaccelerator_accelerator.main.hosted_zone_id
}

output "global_accelerator_ip_sets" {
  description = "IP address sets for the Global Accelerator"
  value       = aws_globalaccelerator_accelerator.main.ip_sets
}

###########################################
# CloudFront Outputs
###########################################

output "cloudfront_distribution_id" {
  description = "ID of the CloudFront distribution"
  value       = aws_cloudfront_distribution.main.id
}

output "cloudfront_distribution_arn" {
  description = "ARN of the CloudFront distribution"
  value       = aws_cloudfront_distribution.main.arn
}

output "cloudfront_domain_name" {
  description = "Domain name of the CloudFront distribution"
  value       = aws_cloudfront_distribution.main.domain_name
}

output "cloudfront_hosted_zone_id" {
  description = "CloudFront hosted zone ID"
  value       = aws_cloudfront_distribution.main.hosted_zone_id
}

###########################################
# SSL Certificate Outputs
###########################################

output "global_ssl_certificate_arn" {
  description = "ARN of the global SSL certificate"
  value       = aws_acm_certificate.global.arn
}

output "global_ssl_certificate_domain" {
  description = "Domain name of the global SSL certificate"
  value       = aws_acm_certificate.global.domain_name
}

###########################################
# S3 Bucket Outputs
###########################################

output "global_logs_bucket" {
  description = "S3 bucket for global logs"
  value       = aws_s3_bucket.global_logs.bucket
}

output "static_assets_bucket" {
  description = "S3 bucket for static assets"
  value       = aws_s3_bucket.static_assets.bucket
}

output "static_assets_bucket_domain" {
  description = "Domain name of the static assets bucket"
  value       = aws_s3_bucket.static_assets.bucket_regional_domain_name
}

###########################################
# Route53 Health Check Outputs
###########################################

output "route53_health_checks" {
  description = "Route53 health check IDs"
  value = {
    primary   = aws_route53_health_check.primary.id
    secondary = aws_route53_health_check.secondary.id
    tertiary  = aws_route53_health_check.tertiary.id
  }
}

###########################################
# Regional Infrastructure Outputs
###########################################

output "regions" {
  description = "Regional infrastructure information"
  value = {
    primary = {
      name                     = local.regions.primary.name
      code                     = local.regions.primary.code
      vpc_id                   = module.primary_region.vpc_id
      vpc_cidr                 = module.primary_region.vpc_cidr_block
      alb_dns_name            = module.primary_region.alb_dns_name
      alb_arn                 = module.primary_region.alb_arn
      eks_cluster_endpoint    = module.primary_region.eks_cluster_endpoint
      eks_cluster_arn         = module.primary_region.eks_cluster_arn
      db_endpoint             = module.primary_region.db_endpoint
      redis_endpoint          = module.primary_region.redis_primary_endpoint
      ssl_certificate_arn     = module.primary_region.ssl_certificate_arn
    }
    secondary = {
      name                     = local.regions.secondary.name
      code                     = local.regions.secondary.code
      vpc_id                   = module.secondary_region.vpc_id
      vpc_cidr                 = module.secondary_region.vpc_cidr_block
      alb_dns_name            = module.secondary_region.alb_dns_name
      alb_arn                 = module.secondary_region.alb_arn
      eks_cluster_endpoint    = module.secondary_region.eks_cluster_endpoint
      eks_cluster_arn         = module.secondary_region.eks_cluster_arn
      db_endpoint             = module.secondary_region.db_endpoint
      redis_endpoint          = module.secondary_region.redis_primary_endpoint
      ssl_certificate_arn     = module.secondary_region.ssl_certificate_arn
    }
    tertiary = {
      name                     = local.regions.tertiary.name
      code                     = local.regions.tertiary.code
      vpc_id                   = module.tertiary_region.vpc_id
      vpc_cidr                 = module.tertiary_region.vpc_cidr_block
      alb_dns_name            = module.tertiary_region.alb_dns_name
      alb_arn                 = module.tertiary_region.alb_arn
      eks_cluster_endpoint    = module.tertiary_region.eks_cluster_endpoint
      eks_cluster_arn         = module.tertiary_region.eks_cluster_arn
      db_endpoint             = module.tertiary_region.db_endpoint
      redis_endpoint          = module.tertiary_region.redis_primary_endpoint
      ssl_certificate_arn     = module.tertiary_region.ssl_certificate_arn
    }
  }
}

###########################################
# Database Configuration Outputs
###########################################

output "database_configuration" {
  description = "Multi-region database configuration"
  value = {
    primary_database = {
      identifier = module.primary_region.db_identifier
      endpoint   = module.primary_region.db_endpoint
      port       = module.primary_region.db_port
      name       = module.primary_region.db_name
      secret_arn = module.primary_region.db_secret_arn
    }
    read_replicas = {
      secondary = {
        identifier = module.secondary_region.db_identifier
        endpoint   = module.secondary_region.db_endpoint
        port       = module.secondary_region.db_port
        secret_arn = module.secondary_region.db_secret_arn
      }
      tertiary = {
        identifier = module.tertiary_region.db_identifier
        endpoint   = module.tertiary_region.db_endpoint
        port       = module.tertiary_region.db_port
        secret_arn = module.tertiary_region.db_secret_arn
      }
    }
  }
  sensitive = true
}

###########################################
# EKS Cluster Configuration Outputs
###########################################

output "eks_clusters" {
  description = "EKS cluster information across regions"
  value = {
    primary = {
      cluster_id       = module.primary_region.eks_cluster_id
      cluster_arn      = module.primary_region.eks_cluster_arn
      cluster_endpoint = module.primary_region.eks_cluster_endpoint
      cluster_version  = module.primary_region.eks_cluster_version
      oidc_issuer_url  = module.primary_region.eks_cluster_oidc_issuer_url
      node_groups      = module.primary_region.eks_node_groups
    }
    secondary = {
      cluster_id       = module.secondary_region.eks_cluster_id
      cluster_arn      = module.secondary_region.eks_cluster_arn
      cluster_endpoint = module.secondary_region.eks_cluster_endpoint
      cluster_version  = module.secondary_region.eks_cluster_version
      oidc_issuer_url  = module.secondary_region.eks_cluster_oidc_issuer_url
      node_groups      = module.secondary_region.eks_node_groups
    }
    tertiary = {
      cluster_id       = module.tertiary_region.eks_cluster_id
      cluster_arn      = module.tertiary_region.eks_cluster_arn
      cluster_endpoint = module.tertiary_region.eks_cluster_endpoint
      cluster_version  = module.tertiary_region.eks_cluster_version
      oidc_issuer_url  = module.tertiary_region.eks_cluster_oidc_issuer_url
      node_groups      = module.tertiary_region.eks_node_groups
    }
  }
}

###########################################
# Cache Configuration Outputs
###########################################

output "redis_clusters" {
  description = "Redis cluster information across regions"
  value = {
    primary = {
      cluster_id     = module.primary_region.redis_cluster_id
      endpoint       = module.primary_region.redis_primary_endpoint
      port           = module.primary_region.redis_port
      secret_arn     = module.primary_region.redis_auth_token_secret_arn
    }
    secondary = {
      cluster_id     = module.secondary_region.redis_cluster_id
      endpoint       = module.secondary_region.redis_primary_endpoint
      port           = module.secondary_region.redis_port
      secret_arn     = module.secondary_region.redis_auth_token_secret_arn
    }
    tertiary = {
      cluster_id     = module.tertiary_region.redis_cluster_id
      endpoint       = module.tertiary_region.redis_primary_endpoint
      port           = module.tertiary_region.redis_port
      secret_arn     = module.tertiary_region.redis_auth_token_secret_arn
    }
  }
  sensitive = true
}

###########################################
# Load Balancer Configuration Outputs
###########################################

output "load_balancers" {
  description = "Load balancer information across regions"
  value = {
    primary = {
      dns_name        = module.primary_region.alb_dns_name
      arn             = module.primary_region.alb_arn
      zone_id         = module.primary_region.alb_zone_id
      target_groups   = module.primary_region.alb_target_groups
    }
    secondary = {
      dns_name        = module.secondary_region.alb_dns_name
      arn             = module.secondary_region.alb_arn
      zone_id         = module.secondary_region.alb_zone_id
      target_groups   = module.secondary_region.alb_target_groups
    }
    tertiary = {
      dns_name        = module.tertiary_region.alb_dns_name
      arn             = module.tertiary_region.alb_arn
      zone_id         = module.tertiary_region.alb_zone_id
      target_groups   = module.tertiary_region.alb_target_groups
    }
  }
}

###########################################
# Networking Configuration Outputs
###########################################

output "networking" {
  description = "Networking configuration across regions"
  value = {
    primary = {
      vpc_id              = module.primary_region.vpc_id
      vpc_cidr            = module.primary_region.vpc_cidr_block
      private_subnet_ids  = module.primary_region.private_subnet_ids
      public_subnet_ids   = module.primary_region.public_subnet_ids
      database_subnet_ids = module.primary_region.database_subnet_ids
      availability_zones  = module.primary_region.availability_zones
    }
    secondary = {
      vpc_id              = module.secondary_region.vpc_id
      vpc_cidr            = module.secondary_region.vpc_cidr_block
      private_subnet_ids  = module.secondary_region.private_subnet_ids
      public_subnet_ids   = module.secondary_region.public_subnet_ids
      database_subnet_ids = module.secondary_region.database_subnet_ids
      availability_zones  = module.secondary_region.availability_zones
    }
    tertiary = {
      vpc_id              = module.tertiary_region.vpc_id
      vpc_cidr            = module.tertiary_region.vpc_cidr_block
      private_subnet_ids  = module.tertiary_region.private_subnet_ids
      public_subnet_ids   = module.tertiary_region.public_subnet_ids
      database_subnet_ids = module.tertiary_region.database_subnet_ids
      availability_zones  = module.tertiary_region.availability_zones
    }
  }
}

###########################################
# Monitoring Configuration Outputs
###########################################

output "monitoring" {
  description = "Monitoring configuration across regions"
  value = {
    primary = {
      cloudwatch_log_groups = module.primary_region.cloudwatch_log_groups
      region               = module.primary_region.region
    }
    secondary = {
      cloudwatch_log_groups = module.secondary_region.cloudwatch_log_groups
      region               = module.secondary_region.region
    }
    tertiary = {
      cloudwatch_log_groups = module.tertiary_region.cloudwatch_log_groups
      region               = module.tertiary_region.region
    }
  }
}

###########################################
# Summary Output
###########################################

output "deployment_summary" {
  description = "High-level summary of the multi-region deployment"
  value = {
    total_regions    = 3
    primary_region   = local.regions.primary.name
    secondary_region = local.regions.secondary.name
    tertiary_region  = local.regions.tertiary.name
    
    global_services = {
      global_accelerator = aws_globalaccelerator_accelerator.main.dns_name
      cloudfront_cdn     = aws_cloudfront_distribution.main.domain_name
      global_certificate = aws_acm_certificate.global.domain_name
    }
    
    deployment_date = timestamp()
    environment     = local.environment
    project_name    = local.project_name
  }
}