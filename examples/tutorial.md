# Getting Started Tutorial

## Introduction

Welcome to the Hotel Reviews Microservice! This tutorial will guide you through setting up the service, understanding its architecture, and performing common operations.

## Prerequisites

Before starting, ensure you have:

- **Go 1.23+** installed
- **Docker & Docker Compose** for infrastructure
- **Git** for version control
- **curl** or **Postman** for API testing
- **Basic understanding** of REST APIs and microservices

## Step 1: Environment Setup

### Clone the Repository

```bash
git clone https://github.com/gkbiswas/hotel-reviews-microservice.git
cd hotel-reviews-microservice
```

### Start Infrastructure Services

```bash
# Start PostgreSQL, Redis, and Kafka
docker-compose up -d postgres redis kafka

# Verify services are running
docker-compose ps
```

Expected output:
```
Name                     Command                  State           Ports
------------------------------------------------------------------------
hotel-reviews_kafka_1    /etc/confluent/docker... Up              9092/tcp
hotel-reviews_postgres_1 docker-entrypoint.sh ... Up              5432/tcp
hotel-reviews_redis_1    docker-entrypoint.sh ... Up              6379/tcp
```

### Install Dependencies

```bash
go mod download
```

## Step 2: Configuration

### Create Configuration File

```bash
mkdir -p config
cat > config/config.yaml << 'EOF'
server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  host: localhost
  port: 5432
  user: postgres
  password: postgres
  name: hotel_reviews
  ssl_mode: disable
  max_conns: 25

redis:
  addr: localhost:6379
  password: ""
  db: 0
  pool_size: 10

kafka:
  brokers: ["localhost:9092"]
  consumer_group: "hotel-reviews-service"

auth:
  jwt_secret: "development-secret-key"
  jwt_expiry: 24h

monitoring:
  enable_metrics: true
  log_level: info
EOF
```

### Set Environment Variables

```bash
export DATABASE_HOST=localhost
export DATABASE_PASSWORD=postgres
export JWT_SECRET=development-secret-key
export LOG_LEVEL=debug
```

## Step 3: Database Setup

### Run Migrations

```bash
# Run database migrations
go run cmd/migrate/main.go up

# Verify tables were created
docker exec -it hotel-reviews_postgres_1 psql -U postgres -d hotel_reviews -c "\dt"
```

### Seed Sample Data

```bash
# Create sample providers
docker exec -it hotel-reviews_postgres_1 psql -U postgres -d hotel_reviews << 'EOF'
INSERT INTO providers (id, name, description, active) VALUES
('550e8400-e29b-41d4-a716-446655440001', 'Booking.com', 'Leading online travel agency', true),
('550e8400-e29b-41d4-a716-446655440002', 'Expedia', 'Global travel technology company', true);

INSERT INTO hotels (id, name, description, city, country, rating) VALUES
('hotel-001', 'Grand Plaza Hotel', 'Luxury hotel in downtown', 'New York', 'USA', 4.5),
('hotel-002', 'Ocean View Resort', 'Beachfront resort with spa', 'Miami', 'USA', 4.2);
EOF
```

## Step 4: Start the Service

### Development Mode

```bash
# Start with hot reload
go run cmd/api/main.go server --config config/config.yaml
```

Expected output:
```
INFO[2024-01-15T10:00:00Z] Starting Hotel Reviews Service
INFO[2024-01-15T10:00:00Z] Database connected successfully
INFO[2024-01-15T10:00:00Z] Redis connected successfully
INFO[2024-01-15T10:00:00Z] Kafka connected successfully
INFO[2024-01-15T10:00:00Z] Server listening on :8080
```

### Verify Service Health

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:00:00Z",
  "checks": {
    "database": "healthy",
    "redis": "healthy",
    "kafka": "healthy"
  }
}
```

## Step 5: API Exploration

### Authentication

First, let's create a user and get an authentication token:

```bash
# Register a new user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123",
    "first_name": "Test",
    "last_name": "User"
  }'

# Login to get token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }'
```

Save the token from the response:
```bash
export TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### Working with Hotels

#### List Hotels

```bash
curl http://localhost:8080/api/v1/hotels
```

#### Get Specific Hotel

```bash
curl http://localhost:8080/api/v1/hotels/hotel-001
```

#### Create New Hotel

```bash
curl -X POST http://localhost:8080/api/v1/hotels \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "Tech Conference Hotel",
    "description": "Modern hotel perfect for business travelers",
    "address": {
      "street": "456 Innovation Drive",
      "city": "San Francisco",
      "state": "CA",
      "country": "USA",
      "postal_code": "94107"
    },
    "amenities": ["wifi", "gym", "parking", "restaurant"],
    "price_range": "$$$"
  }'
```

### Working with Reviews

#### Create a Review

```bash
curl -X POST http://localhost:8080/api/v1/reviews \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "hotel_id": "hotel-001",
    "provider_id": "550e8400-e29b-41d4-a716-446655440001",
    "title": "Excellent stay!",
    "content": "The hotel exceeded all my expectations. Clean rooms, friendly staff, and great location.",
    "rating": 5.0,
    "author_name": "John Doe",
    "author_location": "Boston, MA",
    "review_date": "2024-01-15T10:00:00Z"
  }'
```

#### Get Hotel Reviews

```bash
curl "http://localhost:8080/api/v1/hotels/hotel-001/reviews?limit=10&sort=rating_desc"
```

#### Search Reviews

```bash
curl -X POST http://localhost:8080/api/v1/reviews/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "clean friendly staff",
    "filters": {
      "rating_min": 4.0,
      "language": "en"
    },
    "sort": "review_date_desc",
    "limit": 20
  }'
```

## Step 6: Async Processing

### Bulk Review Upload

Create a sample JSONL file:

```bash
cat > sample_reviews.jsonl << 'EOF'
{"hotel_id": "hotel-001", "provider_id": "550e8400-e29b-41d4-a716-446655440001", "title": "Great service", "content": "Wonderful experience", "rating": 4.5, "author_name": "Alice Smith", "review_date": "2024-01-14T10:00:00Z"}
{"hotel_id": "hotel-001", "provider_id": "550e8400-e29b-41d4-a716-446655440001", "title": "Good location", "content": "Close to everything", "rating": 4.0, "author_name": "Bob Johnson", "review_date": "2024-01-13T15:30:00Z"}
{"hotel_id": "hotel-002", "provider_id": "550e8400-e29b-41d4-a716-446655440002", "title": "Amazing views", "content": "Beautiful ocean views", "rating": 5.0, "author_name": "Carol Davis", "review_date": "2024-01-12T08:45:00Z"}
EOF
```

Upload to MinIO (simulating S3):

```bash
# Start MinIO for local S3 simulation
docker run -d --name minio \
  -p 9000:9000 -p 9001:9001 \
  minio/minio server /data --console-address ":9001"

# Upload file (you would normally upload to S3)
# For this tutorial, we'll simulate with a local file
cp sample_reviews.jsonl /tmp/sample_reviews.jsonl
```

Submit bulk processing job:

```bash
curl -X POST http://localhost:8080/api/v1/reviews/bulk \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "provider_id": "550e8400-e29b-41d4-a716-446655440001",
    "file_url": "file:///tmp/sample_reviews.jsonl",
    "callback_url": "https://webhook.site/your-unique-url"
  }'
```

Check job status:

```bash
# Use the job_id from the previous response
curl http://localhost:8080/api/v1/reviews/bulk/JOB_ID/status \
  -H "Authorization: Bearer $TOKEN"
```

## Step 7: Monitoring and Analytics

### View Metrics

```bash
# Prometheus metrics
curl http://localhost:8080/metrics

# Application metrics
curl http://localhost:8080/api/v1/admin/metrics \
  -H "Authorization: Bearer $TOKEN"
```

### Hotel Analytics

```bash
# Hotel statistics
curl http://localhost:8080/api/v1/analytics/hotels/hotel-001/stats

# Top hotels
curl "http://localhost:8080/api/v1/analytics/top-hotels?limit=10"

# Sentiment analysis
curl -X POST http://localhost:8080/api/v1/analytics/reviews/sentiment \
  -H "Content-Type: application/json" \
  -d '{
    "hotel_id": "hotel-001",
    "date_from": "2024-01-01",
    "date_to": "2024-01-31"
  }'
```

## Step 8: Testing the Cache

### Cache Behavior

```bash
# First request (cache miss)
time curl http://localhost:8080/api/v1/hotels/hotel-001

# Second request (cache hit - should be faster)
time curl http://localhost:8080/api/v1/hotels/hotel-001

# Check cache statistics
curl http://localhost:8080/api/v1/admin/cache/stats \
  -H "Authorization: Bearer $TOKEN"
```

### Cache Management

```bash
# Clear specific cache
curl -X POST http://localhost:8080/api/v1/admin/cache/clear \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"pattern": "hotel:*"}'

# Warm cache
curl -X POST http://localhost:8080/api/v1/admin/cache/warm \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "hotels",
    "criteria": "popular",
    "limit": 50
  }'
```

## Step 9: Error Handling

### Test Rate Limiting

```bash
# Send many requests quickly to trigger rate limit
for i in {1..150}; do
  curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/v1/hotels/hotel-001
done
```

### Test Circuit Breaker

```bash
# Simulate database failure (stop PostgreSQL)
docker-compose stop postgres

# Try to access an endpoint
curl http://localhost:8080/api/v1/hotels/hotel-001

# Check circuit breaker status
curl http://localhost:8080/api/v1/admin/circuit-breakers \
  -H "Authorization: Bearer $TOKEN"

# Restart database
docker-compose start postgres
```

## Step 10: Production Considerations

### Build for Production

```bash
# Build optimized binary
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
  -ldflags '-extldflags "-static"' \
  -o bin/hotel-reviews cmd/api/main.go

# Create Docker image
docker build -t hotel-reviews:latest .
```

### Environment Configuration

```bash
# Production environment variables
export ENVIRONMENT=production
export LOG_LEVEL=warn
export DATABASE_HOST=prod-db.example.com
export REDIS_ADDR=prod-redis.example.com:6379
export KAFKA_BROKERS=prod-kafka1.example.com:9092,prod-kafka2.example.com:9092
export JWT_SECRET=your-production-secret
export ENABLE_METRICS=true
```

### Health Checks

```bash
# Kubernetes readiness probe
curl -f http://localhost:8080/health/ready || exit 1

# Kubernetes liveness probe
curl -f http://localhost:8080/health/live || exit 1

# Deep health check
curl http://localhost:8080/health/deep
```

## Troubleshooting

### Common Issues

1. **Service won't start**
   ```bash
   # Check configuration
   go run cmd/api/main.go server --config config/config.yaml --validate
   
   # Check dependencies
   docker-compose logs postgres redis kafka
   ```

2. **Database connection fails**
   ```bash
   # Test database connectivity
   docker exec -it hotel-reviews_postgres_1 pg_isready
   
   # Check database logs
   docker-compose logs postgres
   ```

3. **High response times**
   ```bash
   # Check cache hit ratio
   curl http://localhost:8080/api/v1/admin/cache/stats
   
   # Monitor database performance
   curl http://localhost:8080/api/v1/admin/database/stats
   ```

### Debug Mode

```bash
# Enable debug logging
DEBUG=true LOG_LEVEL=debug go run cmd/api/main.go server

# Enable SQL query logging
DATABASE_LOG_LEVEL=debug go run cmd/api/main.go server

# Enable profiling
ENABLE_PPROF=true go run cmd/api/main.go server
```

## Next Steps

Now that you have the service running, explore:

1. **[API Documentation](../docs/api.md)** - Complete API reference
2. **[Performance Guide](../docs/performance.md)** - Optimization strategies
3. **[Development Guide](../DEVELOPMENT.md)** - Advanced development topics
4. **[Caching Examples](./caching.md)** - Advanced caching patterns
5. **[Security Examples](./security.md)** - Security implementation details

## Conclusion

You've successfully set up and explored the Hotel Reviews Microservice! The service is now ready for development and testing. The architecture supports high-performance operations with comprehensive caching, async processing, and monitoring capabilities.

For production deployment, refer to the **[Deployment Guide](../docs/DEPLOYMENT_GUIDE.md)** and **[Multi-Region Architecture](../docs/multi-region-architecture.md)** documentation.