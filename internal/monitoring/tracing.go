package monitoring

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingConfig holds configuration for tracing
type TracingConfig struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	JaegerEndpoint string
	SamplingRate   float64
	Enabled        bool
}

// TracingService provides distributed tracing functionality
type TracingService struct {
	tracer trace.Tracer
	config *TracingConfig
	logger *logrus.Logger
}

// NewTracingService creates a new tracing service
func NewTracingService(config *TracingConfig, logger *logrus.Logger) (*TracingService, error) {
	if !config.Enabled {
		logger.Info("Tracing is disabled")
		return &TracingService{
			tracer: trace.NewNoopTracerProvider().Tracer("noop"),
			config: config,
			logger: logger,
		}, nil
	}

	// Validate Jaeger endpoint URL
	parsedURL, err := url.Parse(config.JaegerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: invalid endpoint URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("failed to create Jaeger exporter: invalid URL scheme '%s', expected 'http' or 'https'", parsedURL.Scheme)
	}

	// Create Jaeger exporter
	exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerEndpoint)))
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SamplingRate)),
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer := otel.Tracer(config.ServiceName)

	logger.Info("Tracing initialized successfully",
		logrus.Fields{
			"service":         config.ServiceName,
			"version":         config.ServiceVersion,
			"environment":     config.Environment,
			"jaeger_endpoint": config.JaegerEndpoint,
			"sampling_rate":   config.SamplingRate,
		},
	)

	return &TracingService{
		tracer: tracer,
		config: config,
		logger: logger,
	}, nil
}

// StartSpan starts a new span
func (s *TracingService) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return s.tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the current span from context
func (s *TracingService) SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// WithSpan executes a function with a new span
func (s *TracingService) WithSpan(ctx context.Context, name string, fn func(ctx context.Context, span trace.Span) error) error {
	ctx, span := s.StartSpan(ctx, name)
	defer span.End()

	if err := fn(ctx, span); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// TraceHTTPRequest traces an HTTP request
func (s *TracingService) TraceHTTPRequest(ctx context.Context, method, path, userAgent string, fn func(ctx context.Context, span trace.Span) error) error {
	return s.WithSpan(ctx, fmt.Sprintf("HTTP %s %s", method, path), func(ctx context.Context, span trace.Span) error {
		span.SetAttributes(
			attribute.String("http.method", method),
			attribute.String("http.path", path),
			attribute.String("http.user_agent", userAgent),
			attribute.String("component", "http"),
		)
		return fn(ctx, span)
	})
}

// TraceDatabaseQuery traces a database query
func (s *TracingService) TraceDatabaseQuery(ctx context.Context, table, operation, query string, fn func(ctx context.Context, span trace.Span) error) error {
	return s.WithSpan(ctx, fmt.Sprintf("DB %s %s", operation, table), func(ctx context.Context, span trace.Span) error {
		span.SetAttributes(
			attribute.String("db.table", table),
			attribute.String("db.operation", operation),
			attribute.String("db.statement", query),
			attribute.String("component", "database"),
		)
		return fn(ctx, span)
	})
}

// TraceS3Operation traces an S3 operation
func (s *TracingService) TraceS3Operation(ctx context.Context, operation, bucket, key string, fn func(ctx context.Context, span trace.Span) error) error {
	return s.WithSpan(ctx, fmt.Sprintf("S3 %s", operation), func(ctx context.Context, span trace.Span) error {
		span.SetAttributes(
			attribute.String("s3.operation", operation),
			attribute.String("s3.bucket", bucket),
			attribute.String("s3.key", key),
			attribute.String("component", "s3"),
		)
		return fn(ctx, span)
	})
}

// TraceFileProcessing traces file processing operations
func (s *TracingService) TraceFileProcessing(ctx context.Context, provider, fileURL string, fn func(ctx context.Context, span trace.Span) error) error {
	return s.WithSpan(ctx, "File Processing", func(ctx context.Context, span trace.Span) error {
		span.SetAttributes(
			attribute.String("file.provider", provider),
			attribute.String("file.url", fileURL),
			attribute.String("component", "file_processor"),
		)
		return fn(ctx, span)
	})
}

// TraceReviewProcessing traces review processing
func (s *TracingService) TraceReviewProcessing(ctx context.Context, provider, hotelID string, reviewCount int, fn func(ctx context.Context, span trace.Span) error) error {
	return s.WithSpan(ctx, "Review Processing", func(ctx context.Context, span trace.Span) error {
		span.SetAttributes(
			attribute.String("review.provider", provider),
			attribute.String("review.hotel_id", hotelID),
			attribute.Int("review.count", reviewCount),
			attribute.String("component", "review_processor"),
		)
		return fn(ctx, span)
	})
}

// TraceCacheOperation traces cache operations
func (s *TracingService) TraceCacheOperation(ctx context.Context, operation, key string, hit bool, fn func(ctx context.Context, span trace.Span) error) error {
	return s.WithSpan(ctx, fmt.Sprintf("Cache %s", operation), func(ctx context.Context, span trace.Span) error {
		span.SetAttributes(
			attribute.String("cache.operation", operation),
			attribute.String("cache.key", key),
			attribute.Bool("cache.hit", hit),
			attribute.String("component", "cache"),
		)
		return fn(ctx, span)
	})
}

// TraceCircuitBreakerOperation traces circuit breaker operations
func (s *TracingService) TraceCircuitBreakerOperation(ctx context.Context, component, state string, fn func(ctx context.Context, span trace.Span) error) error {
	return s.WithSpan(ctx, fmt.Sprintf("Circuit Breaker %s", component), func(ctx context.Context, span trace.Span) error {
		span.SetAttributes(
			attribute.String("circuit_breaker.component", component),
			attribute.String("circuit_breaker.state", state),
			attribute.String("component", "circuit_breaker"),
		)
		return fn(ctx, span)
	})
}

// AddSpanAttributes adds attributes to the current span
func (s *TracingService) AddSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// AddSpanEvent adds an event to the current span
func (s *TracingService) AddSpanEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// RecordError records an error in the current span
func (s *TracingService) RecordError(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetSpanStatus sets the status of the current span
func (s *TracingService) SetSpanStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// InjectTraceContext injects trace context into headers
func (s *TracingService) InjectTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// ExtractTraceContext extracts trace context from headers
func (s *TracingService) ExtractTraceContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

// Close closes the tracing service
func (s *TracingService) Close(ctx context.Context) error {
	if !s.config.Enabled {
		return nil
	}

	// Get the trace provider and shut it down
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		return tp.Shutdown(ctx)
	}

	return nil
}

// TraceableFunction is a function that can be traced
type TraceableFunction func(ctx context.Context, span trace.Span) error

// TraceOperation is a generic tracing wrapper for operations
func (s *TracingService) TraceOperation(ctx context.Context, operationName string, attributes []attribute.KeyValue, fn TraceableFunction) error {
	ctx, span := s.StartSpan(ctx, operationName)
	defer span.End()

	if len(attributes) > 0 {
		span.SetAttributes(attributes...)
	}

	if err := fn(ctx, span); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

// MeasureLatency measures the latency of an operation and records it as a span attribute
func (s *TracingService) MeasureLatency(ctx context.Context, operationName string, fn func(ctx context.Context) error) error {
	start := time.Now()
	ctx, span := s.StartSpan(ctx, operationName)
	defer func() {
		latency := time.Since(start)
		span.SetAttributes(attribute.Float64("latency.seconds", latency.Seconds()))
		span.End()
	}()

	if err := fn(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}
