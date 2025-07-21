# Monitoring and Observability Setup Guide

## Overview

This guide covers comprehensive monitoring and observability setup for the Hotel Reviews Microservice, including metrics collection, logging, alerting, and distributed tracing.

## Monitoring Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │    │   Prometheus    │    │   Grafana       │    │   AlertManager  │
│   Services      │    │   (Metrics)     │    │   (Dashboards)  │    │   (Alerts)      │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ Custom Metrics  │───▶│ TSDB Storage    │───▶│ Visualization   │───▶│ PagerDuty       │
│ Business KPIs   │    │ Scraping        │    │ Analytics       │    │ Slack           │
│ System Metrics  │    │ Retention       │    │ Thresholds      │    │ Email           │
└─────────────────┘    └─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │   Jaeger        │    │   ELK Stack     │
                       │   (Tracing)     │    │   (Logging)     │
                       │                 │    │                 │
                       │ Distributed     │    │ Elasticsearch   │
                       │ Request Traces  │    │ Logstash        │
                       │ Performance     │    │ Kibana          │
                       └─────────────────┘    └─────────────────┘
```

## Metrics Implementation

### 1. Custom Business Metrics

```go
package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// Business metrics
var (
    // Review processing metrics
    reviewsProcessedTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "hotel_reviews_processed_total",
            Help: "Total number of reviews processed",
        },
        []string{"provider", "status", "hotel_id"},
    )
    
    reviewProcessingDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "hotel_review_processing_duration_seconds",
            Help: "Time taken to process a review",
            Buckets: prometheus.DefBuckets,
        },
        []string{"provider", "processing_type"},
    )
    
    // Hotel metrics
    hotelRatingUpdates = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "hotel_rating_updates_total",
            Help: "Total number of hotel rating updates",
        },
        []string{"hotel_id", "trigger"},
    )
    
    averageHotelRating = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "hotel_average_rating",
            Help: "Current average rating for hotels",
        },
        []string{"hotel_id", "city", "country"},
    )
    
    // API metrics
    apiRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "api_request_duration_seconds",
            Help: "API request duration in seconds",
            Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
        },
        []string{"method", "endpoint", "status_code"},
    )
    
    apiRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "api_requests_total",
            Help: "Total number of API requests",
        },
        []string{"method", "endpoint", "status_code"},
    )
    
    // Cache metrics
    cacheOperations = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_operations_total",
            Help: "Total number of cache operations",
        },
        []string{"cache_type", "operation", "result"},
    )
    
    cacheHitRatio = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "cache_hit_ratio",
            Help: "Cache hit ratio by cache type",
        },
        []string{"cache_type"},
    )
    
    // Database metrics
    databaseQueries = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "database_queries_total",
            Help: "Total number of database queries",
        },
        []string{"query_type", "table", "status"},
    )
    
    databaseQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "database_query_duration_seconds",
            Help: "Database query duration in seconds",
            Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
        },
        []string{"query_type", "table"},
    )
    
    databaseConnections = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "database_connections",
            Help: "Number of database connections",
        },
        []string{"state"},
    )
    
    // Bulk processing metrics
    bulkJobsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bulk_jobs_total",
            Help: "Total number of bulk processing jobs",
        },
        []string{"provider", "status"},
    )
    
    bulkJobDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "bulk_job_duration_seconds",
            Help: "Bulk job processing duration in seconds",
            Buckets: []float64{1, 5, 10, 30, 60, 300, 600, 1200, 1800, 3600},
        },
        []string{"provider", "job_type"},
    )
    
    bulkRecordsProcessed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "bulk_records_processed_total",
            Help: "Total number of records processed in bulk jobs",
        },
        []string{"provider", "status"},
    )
)

// Metrics service for business logic instrumentation
type MetricsService struct {
    registry prometheus.Registerer
}

func NewMetricsService() *MetricsService {
    return &MetricsService{
        registry: prometheus.DefaultRegisterer,
    }
}

func (ms *MetricsService) RecordReviewProcessed(provider, status, hotelID string, duration float64) {
    reviewsProcessedTotal.WithLabelValues(provider, status, hotelID).Inc()
    reviewProcessingDuration.WithLabelValues(provider, "single").Observe(duration)
}

func (ms *MetricsService) RecordHotelRatingUpdate(hotelID, city, country string, rating float64, trigger string) {
    hotelRatingUpdates.WithLabelValues(hotelID, trigger).Inc()
    averageHotelRating.WithLabelValues(hotelID, city, country).Set(rating)
}

func (ms *MetricsService) RecordAPIRequest(method, endpoint, statusCode string, duration float64) {
    apiRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
    apiRequestDuration.WithLabelValues(method, endpoint, statusCode).Observe(duration)
}

func (ms *MetricsService) RecordCacheOperation(cacheType, operation, result string) {
    cacheOperations.WithLabelValues(cacheType, operation, result).Inc()
}

func (ms *MetricsService) UpdateCacheHitRatio(cacheType string, ratio float64) {
    cacheHitRatio.WithLabelValues(cacheType).Set(ratio)
}

func (ms *MetricsService) RecordDatabaseQuery(queryType, table, status string, duration float64) {
    databaseQueries.WithLabelValues(queryType, table, status).Inc()
    databaseQueryDuration.WithLabelValues(queryType, table).Observe(duration)
}

func (ms *MetricsService) UpdateDatabaseConnections(activeConns, idleConns, totalConns int) {
    databaseConnections.WithLabelValues("active").Set(float64(activeConns))
    databaseConnections.WithLabelValues("idle").Set(float64(idleConns))
    databaseConnections.WithLabelValues("total").Set(float64(totalConns))
}

func (ms *MetricsService) RecordBulkJob(provider, status, jobType string, duration float64, recordsProcessed int, recordsFailed int) {
    bulkJobsTotal.WithLabelValues(provider, status).Inc()
    bulkJobDuration.WithLabelValues(provider, jobType).Observe(duration)
    bulkRecordsProcessed.WithLabelValues(provider, "success").Add(float64(recordsProcessed))
    bulkRecordsProcessed.WithLabelValues(provider, "failed").Add(float64(recordsFailed))
}
```

### 2. Middleware for Automatic Instrumentation

```go
package middleware

import (
    "time"
    "github.com/gin-gonic/gin"
    "strconv"
)

func PrometheusMiddleware(metricsService *MetricsService) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        // Process request
        c.Next()
        
        // Record metrics
        duration := time.Since(start).Seconds()
        statusCode := strconv.Itoa(c.Writer.Status())
        
        metricsService.RecordAPIRequest(
            c.Request.Method,
            c.FullPath(),
            statusCode,
            duration,
        )
        
        // Record additional business metrics based on endpoint
        switch c.FullPath() {
        case "/api/v1/reviews":
            if c.Request.Method == "POST" && c.Writer.Status() == 201 {
                // Extract hotel_id and provider from request if needed
                metricsService.RecordReviewProcessed("api", "created", "unknown", duration)
            }
        case "/api/v1/reviews/bulk":
            if c.Request.Method == "POST" {
                metricsService.RecordBulkJob("api", "submitted", "review_import", duration, 0, 0)
            }
        }
    }
}
```

## Logging Strategy

### 1. Structured Logging Setup

```go
package logging

import (
    "context"
    "os"
    "github.com/sirupsen/logrus"
    "github.com/google/uuid"
)

type Logger struct {
    *logrus.Logger
}

type ContextKey string

const (
    RequestIDKey  ContextKey = "request_id"
    CorrelationKey ContextKey = "correlation_id"
    UserIDKey     ContextKey = "user_id"
)

func NewLogger() *Logger {
    log := logrus.New()
    
    // Set JSON formatter for structured logging
    log.SetFormatter(&logrus.JSONFormatter{
        TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
        FieldMap: logrus.FieldMap{
            logrus.FieldKeyTime:  "timestamp",
            logrus.FieldKeyLevel: "level",
            logrus.FieldKeyMsg:   "message",
        },
    })
    
    // Set log level from environment
    level := os.Getenv("LOG_LEVEL")
    switch level {
    case "debug":
        log.SetLevel(logrus.DebugLevel)
    case "info":
        log.SetLevel(logrus.InfoLevel)
    case "warn":
        log.SetLevel(logrus.WarnLevel)
    case "error":
        log.SetLevel(logrus.ErrorLevel)
    default:
        log.SetLevel(logrus.InfoLevel)
    }
    
    return &Logger{log}
}

func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
    entry := l.WithFields(logrus.Fields{})
    
    if requestID := ctx.Value(RequestIDKey); requestID != nil {
        entry = entry.WithField("request_id", requestID)
    }
    
    if correlationID := ctx.Value(CorrelationKey); correlationID != nil {
        entry = entry.WithField("correlation_id", correlationID)
    }
    
    if userID := ctx.Value(UserIDKey); userID != nil {
        entry = entry.WithField("user_id", userID)
    }
    
    return entry
}

func (l *Logger) LogBusinessEvent(ctx context.Context, event string, data map[string]interface{}) {
    entry := l.WithContext(ctx).WithFields(logrus.Fields{
        "event_type": "business_event",
        "event_name": event,
    })
    
    for k, v := range data {
        entry = entry.WithField(k, v)
    }
    
    entry.Info("Business event occurred")
}

func (l *Logger) LogSecurityEvent(ctx context.Context, event string, severity string, data map[string]interface{}) {
    entry := l.WithContext(ctx).WithFields(logrus.Fields{
        "event_type": "security_event",
        "event_name": event,
        "severity": severity,
    })
    
    for k, v := range data {
        entry = entry.WithField(k, v)
    }
    
    entry.Warn("Security event detected")
}

func (l *Logger) LogPerformanceMetric(ctx context.Context, operation string, duration float64, metadata map[string]interface{}) {
    entry := l.WithContext(ctx).WithFields(logrus.Fields{
        "event_type": "performance_metric",
        "operation": operation,
        "duration_ms": duration * 1000,
    })
    
    for k, v := range metadata {
        entry = entry.WithField(k, v)
    }
    
    if duration > 1.0 { // Log as warning if operation took more than 1 second
        entry.Warn("Slow operation detected")
    } else {
        entry.Debug("Performance metric recorded")
    }
}
```

### 2. Logging Middleware

```go
func LoggingMiddleware(logger *logging.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        // Generate request ID if not present
        requestID := c.GetHeader("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
            c.Header("X-Request-ID", requestID)
        }
        
        // Add to context
        ctx := context.WithValue(c.Request.Context(), logging.RequestIDKey, requestID)
        c.Request = c.Request.WithContext(ctx)
        
        // Log request start
        logger.WithContext(ctx).WithFields(logrus.Fields{
            "method": c.Request.Method,
            "path": c.Request.URL.Path,
            "query": c.Request.URL.RawQuery,
            "ip": c.ClientIP(),
            "user_agent": c.Request.UserAgent(),
        }).Info("Request started")
        
        // Process request
        c.Next()
        
        // Log request completion
        duration := time.Since(start)
        statusCode := c.Writer.Status()
        
        logLevel := logrus.InfoLevel
        if statusCode >= 400 {
            logLevel = logrus.WarnLevel
        }
        if statusCode >= 500 {
            logLevel = logrus.ErrorLevel
        }
        
        logger.WithContext(ctx).WithFields(logrus.Fields{
            "method": c.Request.Method,
            "path": c.Request.URL.Path,
            "status_code": statusCode,
            "duration_ms": float64(duration.Nanoseconds()) / 1e6,
            "response_size": c.Writer.Size(),
        }).Log(logLevel, "Request completed")
    }
}
```

## Distributed Tracing

### 1. OpenTelemetry Setup

```go
package tracing

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    "go.opentelemetry.io/otel/semconv/v1.10.0/semconv"
)

func InitTracing(serviceName, jaegerEndpoint string) (*trace.TracerProvider, error) {
    // Create Jaeger exporter
    exp, err := jaeger.New(
        jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(jaegerEndpoint)),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
    }
    
    // Create resource
    res, err := resource.New(
        context.Background(),
        resource.WithAttributes(
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String("1.0.0"),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create resource: %w", err)
    }
    
    // Create tracer provider
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exp),
        trace.WithResource(res),
        trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(0.1))), // 10% sampling
    )
    
    // Set global tracer provider
    otel.SetTracerProvider(tp)
    
    // Set global propagator
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))
    
    return tp, nil
}

// Tracing middleware for Gin
func TracingMiddleware() gin.HandlerFunc {
    tracer := otel.Tracer("hotel-reviews-api")
    
    return func(c *gin.Context) {
        // Extract tracing context from headers
        ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
        
        // Start new span
        ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", c.Request.Method, c.FullPath()))
        defer span.End()
        
        // Add span attributes
        span.SetAttributes(
            semconv.HTTPMethodKey.String(c.Request.Method),
            semconv.HTTPURLKey.String(c.Request.URL.String()),
            semconv.HTTPUserAgentKey.String(c.Request.UserAgent()),
            semconv.NetPeerIPKey.String(c.ClientIP()),
        )
        
        // Update request context
        c.Request = c.Request.WithContext(ctx)
        
        // Process request
        c.Next()
        
        // Update span with response info
        span.SetAttributes(
            semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
            semconv.HTTPResponseSizeKey.Int(c.Writer.Size()),
        )
        
        // Set span status based on HTTP status
        if c.Writer.Status() >= 400 {
            span.RecordError(fmt.Errorf("HTTP %d", c.Writer.Status()))
        }
    }
}
```

### 2. Database Tracing

```go
package database

import (
    "context"
    "database/sql/driver"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

type TracingWrapper struct {
    driver.Conn
    tracer trace.Tracer
}

func (tw *TracingWrapper) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
    ctx, span := tw.tracer.Start(ctx, "db.query")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("db.statement", query),
        attribute.String("db.operation", "query"),
    )
    
    rows, err := tw.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
    if err != nil {
        span.RecordError(err)
    }
    
    return rows, err
}

func (tw *TracingWrapper) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
    ctx, span := tw.tracer.Start(ctx, "db.exec")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("db.statement", query),
        attribute.String("db.operation", "exec"),
    )
    
    result, err := tw.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
    if err != nil {
        span.RecordError(err)
    }
    
    return result, err
}
```

## Health Checks

### 1. Multi-Level Health Checks

```go
package health

import (
    "context"
    "database/sql"
    "encoding/json"
    "net/http"
    "time"
    "github.com/go-redis/redis/v8"
)

type HealthChecker struct {
    db          *sql.DB
    redisClient *redis.Client
    dependencies []Dependency
}

type Dependency struct {
    Name string
    CheckFunc func(ctx context.Context) error
}

type HealthStatus struct {
    Status    string                 `json:"status"`
    Timestamp time.Time              `json:"timestamp"`
    Version   string                 `json:"version"`
    Uptime    string                 `json:"uptime"`
    Checks    map[string]CheckResult `json:"checks"`
}

type CheckResult struct {
    Status      string        `json:"status"`
    Duration    time.Duration `json:"duration"`
    Error       string        `json:"error,omitempty"`
    LastChecked time.Time     `json:"last_checked"`
}

func NewHealthChecker(db *sql.DB, redisClient *redis.Client) *HealthChecker {
    hc := &HealthChecker{
        db:          db,
        redisClient: redisClient,
    }
    
    // Register core dependencies
    hc.dependencies = []Dependency{
        {
            Name: "database",
            CheckFunc: hc.checkDatabase,
        },
        {
            Name: "redis",
            CheckFunc: hc.checkRedis,
        },
        {
            Name: "external_api",
            CheckFunc: hc.checkExternalAPI,
        },
    }
    
    return hc
}

func (hc *HealthChecker) checkDatabase(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    return hc.db.PingContext(ctx)
}

func (hc *HealthChecker) checkRedis(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    return hc.redisClient.Ping(ctx).Err()
}

func (hc *HealthChecker) checkExternalAPI(ctx context.Context) error {
    // Implement external service health checks
    ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
    defer cancel()
    
    client := &http.Client{Timeout: 10 * time.Second}
    req, err := http.NewRequestWithContext(ctx, "GET", "https://api.external-service.com/health", nil)
    if err != nil {
        return err
    }
    
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("external API returned status %d", resp.StatusCode)
    }
    
    return nil
}

func (hc *HealthChecker) CheckHealth(ctx context.Context) HealthStatus {
    startTime := time.Now()
    checks := make(map[string]CheckResult)
    overallStatus := "healthy"
    
    for _, dep := range hc.dependencies {
        checkStart := time.Now()
        err := dep.CheckFunc(ctx)
        duration := time.Since(checkStart)
        
        result := CheckResult{
            Duration:    duration,
            LastChecked: checkStart,
        }
        
        if err != nil {
            result.Status = "unhealthy"
            result.Error = err.Error()
            overallStatus = "unhealthy"
        } else {
            result.Status = "healthy"
        }
        
        checks[dep.Name] = result
    }
    
    return HealthStatus{
        Status:    overallStatus,
        Timestamp: startTime,
        Version:   "1.0.0",
        Uptime:    time.Since(startTime).String(),
        Checks:    checks,
    }
}

// HTTP handlers for different health check endpoints
func (hc *HealthChecker) LivenessHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "status": "alive",
            "timestamp": time.Now(),
        })
    }
}

func (hc *HealthChecker) ReadinessHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
        defer cancel()
        
        health := hc.CheckHealth(ctx)
        
        statusCode := http.StatusOK
        if health.Status != "healthy" {
            statusCode = http.StatusServiceUnavailable
        }
        
        c.JSON(statusCode, health)
    }
}

func (hc *HealthChecker) DeepHealthHandler() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
        defer cancel()
        
        // Perform comprehensive health checks
        health := hc.CheckHealth(ctx)
        
        // Add additional system metrics
        health.Checks["system"] = hc.getSystemMetrics()
        
        statusCode := http.StatusOK
        if health.Status != "healthy" {
            statusCode = http.StatusServiceUnavailable
        }
        
        c.JSON(statusCode, health)
    }
}

func (hc *HealthChecker) getSystemMetrics() CheckResult {
    // Get system metrics like memory, CPU, disk usage
    return CheckResult{
        Status:      "healthy",
        Duration:    time.Millisecond * 10,
        LastChecked: time.Now(),
    }
}
```

## Docker Compose for Local Monitoring

### 1. Monitoring Stack

```yaml
# docker-compose.monitoring.yml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - ./monitoring/alerts.yml:/etc/prometheus/alerts.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=200h'
      - '--web.enable-lifecycle'
    networks:
      - monitoring

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/grafana/provisioning:/etc/grafana/provisioning
      - ./monitoring/grafana/dashboards:/var/lib/grafana/dashboards
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    networks:
      - monitoring

  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: jaeger
    ports:
      - "16686:16686"
      - "14268:14268"
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    networks:
      - monitoring

  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:7.17.0
    container_name: elasticsearch
    environment:
      - discovery.type=single-node
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
    ports:
      - "9200:9200"
    volumes:
      - elasticsearch_data:/usr/share/elasticsearch/data
    networks:
      - monitoring

  kibana:
    image: docker.elastic.co/kibana/kibana:7.17.0
    container_name: kibana
    ports:
      - "5601:5601"
    environment:
      - ELASTICSEARCH_HOSTS=http://elasticsearch:9200
    depends_on:
      - elasticsearch
    networks:
      - monitoring

  logstash:
    image: docker.elastic.co/logstash/logstash:7.17.0
    container_name: logstash
    ports:
      - "5044:5044"
    volumes:
      - ./monitoring/logstash/pipeline:/usr/share/logstash/pipeline
      - ./monitoring/logstash/config:/usr/share/logstash/config
    depends_on:
      - elasticsearch
    networks:
      - monitoring

  alertmanager:
    image: prom/alertmanager:latest
    container_name: alertmanager
    ports:
      - "9093:9093"
    volumes:
      - ./monitoring/alertmanager.yml:/etc/alertmanager/alertmanager.yml
    networks:
      - monitoring

volumes:
  prometheus_data:
  grafana_data:
  elasticsearch_data:

networks:
  monitoring:
    driver: bridge
```

### 2. Prometheus Configuration

```yaml
# monitoring/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "alerts.yml"

scrape_configs:
  - job_name: 'hotel-reviews'
    static_configs:
      - targets: ['host.docker.internal:8080']
    metrics_path: '/metrics'
    scrape_interval: 10s

  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']

  - job_name: 'node-exporter'
    static_configs:
      - targets: ['node-exporter:9100']

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

### 3. Grafana Dashboards

```json
{
  "dashboard": {
    "id": null,
    "title": "Hotel Reviews Microservice",
    "tags": ["hotel-reviews"],
    "timezone": "browser",
    "panels": [
      {
        "id": 1,
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(api_requests_total[5m])",
            "legendFormat": "{{method}} {{endpoint}}"
          }
        ]
      },
      {
        "id": 2,
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(api_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          },
          {
            "expr": "histogram_quantile(0.50, rate(api_request_duration_seconds_bucket[5m]))",
            "legendFormat": "50th percentile"
          }
        ]
      },
      {
        "id": 3,
        "title": "Error Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(api_requests_total{status_code=~\"4..|5..\"}[5m])",
            "legendFormat": "Error Rate"
          }
        ]
      },
      {
        "id": 4,
        "title": "Cache Hit Ratio",
        "type": "stat",
        "targets": [
          {
            "expr": "cache_hit_ratio",
            "legendFormat": "{{cache_type}}"
          }
        ]
      }
    ],
    "time": {
      "from": "now-1h",
      "to": "now"
    },
    "refresh": "5s"
  }
}
```

This comprehensive monitoring guide provides everything needed to implement world-class observability for the Hotel Reviews Microservice.