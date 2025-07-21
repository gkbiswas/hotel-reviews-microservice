# Application Load Balancer Configuration
# Regional load balancer for microservices routing

###########################################
# Application Load Balancer
###########################################

resource "aws_lb" "main" {
  name               = "${var.project_name}-${local.region_code}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = aws_subnet.public[*].id

  enable_deletion_protection       = var.is_primary ? true : false
  enable_cross_zone_load_balancing = true
  enable_http2                     = true
  idle_timeout                     = 60

  # Access logs
  access_logs {
    bucket  = aws_s3_bucket.alb_logs.bucket
    prefix  = "alb-access-logs"
    enabled = true
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-alb"
    Type = "ApplicationLoadBalancer"
  })
}

###########################################
# S3 Bucket for ALB Access Logs
###########################################

resource "aws_s3_bucket" "alb_logs" {
  bucket = "${var.project_name}-${local.region_code}-alb-logs-${random_id.alb_logs_suffix.hex}"

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-alb-logs"
  })
}

resource "aws_s3_bucket_versioning" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_encryption" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  server_side_encryption_configuration {
    rule {
      apply_server_side_encryption_by_default {
        sse_algorithm = "AES256"
      }
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  rule {
    id     = "alb_logs_lifecycle"
    status = "Enabled"

    expiration {
      days = 90
    }

    noncurrent_version_expiration {
      noncurrent_days = 30
    }
  }
}

resource "aws_s3_bucket_policy" "alb_logs" {
  bucket = aws_s3_bucket.alb_logs.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Principal = {
          AWS = data.aws_elb_service_account.main.arn
        }
        Action   = "s3:PutObject"
        Resource = "${aws_s3_bucket.alb_logs.arn}/alb-access-logs/AWSLogs/${data.aws_caller_identity.current.account_id}/*"
      },
      {
        Effect = "Allow"
        Principal = {
          Service = "delivery.logs.amazonaws.com"
        }
        Action   = "s3:PutObject"
        Resource = "${aws_s3_bucket.alb_logs.arn}/alb-access-logs/AWSLogs/${data.aws_caller_identity.current.account_id}/*"
        Condition = {
          StringEquals = {
            "s3:x-amz-acl" = "bucket-owner-full-control"
          }
        }
      },
      {
        Effect = "Allow"
        Principal = {
          Service = "delivery.logs.amazonaws.com"
        }
        Action   = "s3:GetBucketAcl"
        Resource = aws_s3_bucket.alb_logs.arn
      }
    ]
  })
}

resource "random_id" "alb_logs_suffix" {
  byte_length = 4
}

data "aws_elb_service_account" "main" {}

###########################################
# ALB Target Groups
###########################################

# API Gateway Target Group
resource "aws_lb_target_group" "api_gateway" {
  name     = "${var.project_name}-${local.region_code}-api-tg"
  port     = 8080
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  stickiness {
    type            = "lb_cookie"
    cookie_duration = 86400
    enabled         = false
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-api-tg"
  })
}

# User Service Target Group
resource "aws_lb_target_group" "user_service" {
  name     = "${var.project_name}-${local.region_code}-user-tg"
  port     = 8081
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-user-tg"
  })
}

# Hotel Service Target Group
resource "aws_lb_target_group" "hotel_service" {
  name     = "${var.project_name}-${local.region_code}-hotel-tg"
  port     = 8082
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-hotel-tg"
  })
}

# Review Service Target Group
resource "aws_lb_target_group" "review_service" {
  name     = "${var.project_name}-${local.region_code}-review-tg"
  port     = 8083
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-review-tg"
  })
}

# Analytics Service Target Group
resource "aws_lb_target_group" "analytics_service" {
  name     = "${var.project_name}-${local.region_code}-analytics-tg"
  port     = 8084
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-analytics-tg"
  })
}

# Notification Service Target Group
resource "aws_lb_target_group" "notification_service" {
  name     = "${var.project_name}-${local.region_code}-notif-tg"
  port     = 8085
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-notif-tg"
  })
}

# File Processing Service Target Group
resource "aws_lb_target_group" "file_service" {
  name     = "${var.project_name}-${local.region_code}-file-tg"
  port     = 8086
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-file-tg"
  })
}

# Search Service Target Group
resource "aws_lb_target_group" "search_service" {
  name     = "${var.project_name}-${local.region_code}-search-tg"
  port     = 8087
  protocol = "HTTP"
  vpc_id   = aws_vpc.main.id

  health_check {
    enabled             = true
    healthy_threshold   = 2
    interval            = 30
    matcher             = "200"
    path                = "/health"
    port                = "traffic-port"
    protocol            = "HTTP"
    timeout             = 5
    unhealthy_threshold = 3
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-search-tg"
  })
}

###########################################
# SSL Certificate
###########################################

resource "aws_acm_certificate" "main" {
  domain_name       = "${local.region_code}.${var.domain_name}"
  validation_method = "DNS"

  subject_alternative_names = [
    "api.${local.region_code}.${var.domain_name}",
    "*.${local.region_code}.${var.domain_name}"
  ]

  lifecycle {
    create_before_destroy = true
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${local.region_code}-cert"
  })
}

###########################################
# ALB Listeners
###########################################

# HTTP Listener (Redirect to HTTPS)
resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type = "redirect"

    redirect {
      port        = "443"
      protocol    = "HTTPS"
      status_code = "HTTP_301"
    }
  }

  tags = local.common_tags
}

# HTTPS Listener
resource "aws_lb_listener" "https" {
  load_balancer_arn = aws_lb.main.arn
  port              = "443"
  protocol          = "HTTPS"
  ssl_policy        = "ELBSecurityPolicy-TLS-1-2-2017-01"
  certificate_arn   = aws_acm_certificate.main.arn

  # Default action returns 404 for unmatched requests
  default_action {
    type = "fixed-response"

    fixed_response {
      content_type = "text/plain"
      message_body = "Not Found"
      status_code  = "404"
    }
  }

  tags = local.common_tags
}

###########################################
# ALB Listener Rules
###########################################

# API Gateway routing
resource "aws_lb_listener_rule" "api_gateway" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 100

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.api_gateway.arn
  }

  condition {
    path_pattern {
      values = ["/api/*"]
    }
  }

  tags = local.common_tags
}

# Health check routing
resource "aws_lb_listener_rule" "health" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 50

  action {
    type = "fixed-response"

    fixed_response {
      content_type = "text/plain"
      message_body = "OK"
      status_code  = "200"
    }
  }

  condition {
    path_pattern {
      values = ["/health"]
    }
  }

  tags = local.common_tags
}

# User service routing
resource "aws_lb_listener_rule" "user_service" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 200

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.user_service.arn
  }

  condition {
    path_pattern {
      values = ["/api/users/*", "/api/auth/*"]
    }
  }

  tags = local.common_tags
}

# Hotel service routing
resource "aws_lb_listener_rule" "hotel_service" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 300

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.hotel_service.arn
  }

  condition {
    path_pattern {
      values = ["/api/hotels/*"]
    }
  }

  tags = local.common_tags
}

# Review service routing
resource "aws_lb_listener_rule" "review_service" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 400

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.review_service.arn
  }

  condition {
    path_pattern {
      values = ["/api/reviews/*"]
    }
  }

  tags = local.common_tags
}

# Analytics service routing
resource "aws_lb_listener_rule" "analytics_service" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 500

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.analytics_service.arn
  }

  condition {
    path_pattern {
      values = ["/api/analytics/*"]
    }
  }

  tags = local.common_tags
}

# Notification service routing
resource "aws_lb_listener_rule" "notification_service" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 600

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.notification_service.arn
  }

  condition {
    path_pattern {
      values = ["/api/notifications/*"]
    }
  }

  tags = local.common_tags
}

# File processing service routing
resource "aws_lb_listener_rule" "file_service" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 700

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.file_service.arn
  }

  condition {
    path_pattern {
      values = ["/api/files/*", "/api/upload/*"]
    }
  }

  tags = local.common_tags
}

# Search service routing
resource "aws_lb_listener_rule" "search_service" {
  listener_arn = aws_lb_listener.https.arn
  priority     = 800

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.search_service.arn
  }

  condition {
    path_pattern {
      values = ["/api/search/*"]
    }
  }

  tags = local.common_tags
}