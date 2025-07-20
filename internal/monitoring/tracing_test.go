package monitoring

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestNewTracingService_Disabled(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		JaegerEndpoint: "http://localhost:14268/api/traces",
		SamplingRate:   1.0,
		Enabled:        false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, config, service.config)
	assert.NotNil(t, service.tracer)
	assert.NotNil(t, service.logger)
}

func TestNewTracingService_Enabled_InvalidEndpoint(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		JaegerEndpoint: "invalid://endpoint",
		SamplingRate:   1.0,
		Enabled:        true,
	}

	_, err := NewTracingService(config, logger)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create Jaeger exporter")
}

func TestTracingService_StartSpan(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		Enabled:        false, // Use noop tracer for testing
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	newCtx, span := service.StartSpan(ctx, "test-span")

	assert.NotNil(t, newCtx)
	assert.NotNil(t, span)
	assert.NotEqual(t, ctx, newCtx)
	
	// Clean up
	span.End()
}

func TestTracingService_SpanFromContext(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := service.StartSpan(ctx, "test-span")
	defer span.End()

	retrievedSpan := service.SpanFromContext(ctx)
	assert.NotNil(t, retrievedSpan)
	assert.Equal(t, span, retrievedSpan)
}

func TestTracingService_WithSpan_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.WithSpan(ctx, "test-operation", func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_WithSpan_Error(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	testError := errors.New("test error")

	err = service.WithSpan(ctx, "test-operation", func(ctx context.Context, span trace.Span) error {
		return testError
	})

	assert.Error(t, err)
	assert.Equal(t, testError, err)
}

func TestTracingService_TraceHTTPRequest(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceHTTPRequest(ctx, "GET", "/api/reviews", "test-agent", func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_TraceDatabaseQuery(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceDatabaseQuery(ctx, "reviews", "SELECT", "SELECT * FROM reviews", func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_TraceS3Operation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceS3Operation(ctx, "GetObject", "test-bucket", "test-key", func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_TraceFileProcessing(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceFileProcessing(ctx, "booking.com", "https://example.com/file.csv", func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_TraceReviewProcessing(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceReviewProcessing(ctx, "booking.com", "hotel-123", 100, func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_TraceCacheOperation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceCacheOperation(ctx, "GET", "review:123", true, func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_TraceCircuitBreakerOperation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.TraceCircuitBreakerOperation(ctx, "database", "closed", func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_AddSpanAttributes(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := service.StartSpan(ctx, "test-span")
	defer span.End()

	// This should not panic with noop tracer
	service.AddSpanAttributes(ctx, 
		attribute.String("test.key", "test.value"),
		attribute.Int("test.count", 42),
	)
}

func TestTracingService_AddSpanEvent(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := service.StartSpan(ctx, "test-span")
	defer span.End()

	// This should not panic with noop tracer
	service.AddSpanEvent(ctx, "test-event",
		attribute.String("event.type", "test"),
		attribute.Int("event.count", 1),
	)
}

func TestTracingService_RecordError(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := service.StartSpan(ctx, "test-span")
	defer span.End()

	testError := errors.New("test error")
	
	// This should not panic with noop tracer
	service.RecordError(ctx, testError)
}

func TestTracingService_SetSpanStatus(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := service.StartSpan(ctx, "test-span")
	defer span.End()

	// This should not panic with noop tracer
	service.SetSpanStatus(ctx, codes.Ok, "success")
	service.SetSpanStatus(ctx, codes.Error, "failure")
}

func TestTracingService_TraceContextPropagation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := service.StartSpan(ctx, "test-span")
	defer span.End()

	// Test injection
	headers := http.Header{}
	carrier := propagation.HeaderCarrier(headers)
	
	// This should not panic with noop tracer
	service.InjectTraceContext(ctx, carrier)

	// Test extraction
	newCtx := service.ExtractTraceContext(context.Background(), carrier)
	assert.NotNil(t, newCtx)
}

func TestTracingService_Close_Disabled(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	err = service.Close(ctx)
	assert.NoError(t, err)
}

func TestTracingService_TraceOperation_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	attrs := []attribute.KeyValue{
		attribute.String("operation.type", "test"),
		attribute.Int("operation.count", 1),
	}

	err = service.TraceOperation(ctx, "test-operation", attrs, func(ctx context.Context, span trace.Span) error {
		executed = true
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_TraceOperation_Error(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	testError := errors.New("operation failed")

	attrs := []attribute.KeyValue{
		attribute.String("operation.type", "test"),
	}

	err = service.TraceOperation(ctx, "test-operation", attrs, func(ctx context.Context, span trace.Span) error {
		return testError
	})

	assert.Error(t, err)
	assert.Equal(t, testError, err)
}

func TestTracingService_MeasureLatency_Success(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	executed := false

	err = service.MeasureLatency(ctx, "latency-test", func(ctx context.Context) error {
		executed = true
		// Simulate some work
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	assert.NoError(t, err)
	assert.True(t, executed)
}

func TestTracingService_MeasureLatency_Error(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	testError := errors.New("latency test failed")

	err = service.MeasureLatency(ctx, "latency-test", func(ctx context.Context) error {
		time.Sleep(5 * time.Millisecond)
		return testError
	})

	assert.Error(t, err)
	assert.Equal(t, testError, err)
}

func TestTracingConfig_Validation(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	tests := []struct {
		name   string
		config *TracingConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: &TracingConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				JaegerEndpoint: "http://localhost:14268/api/traces",
				SamplingRate:   0.1,
				Enabled:        false,
			},
			valid: true,
		},
		{
			name: "empty service name",
			config: &TracingConfig{
				ServiceName:    "",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				JaegerEndpoint: "http://localhost:14268/api/traces",
				SamplingRate:   0.1,
				Enabled:        false,
			},
			valid: true, // Service name can be empty for noop tracer
		},
		{
			name: "zero sampling rate",
			config: &TracingConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				JaegerEndpoint: "http://localhost:14268/api/traces",
				SamplingRate:   0.0,
				Enabled:        false,
			},
			valid: true,
		},
		{
			name: "max sampling rate",
			config: &TracingConfig{
				ServiceName:    "test-service",
				ServiceVersion: "1.0.0",
				Environment:    "test",
				JaegerEndpoint: "http://localhost:14268/api/traces",
				SamplingRate:   1.0,
				Enabled:        false,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewTracingService(tt.config, logger)
			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, service)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestTracingService_NestedSpans(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	outerExecuted := false
	innerExecuted := false

	err = service.WithSpan(ctx, "outer-span", func(outerCtx context.Context, outerSpan trace.Span) error {
		outerExecuted = true
		assert.NotNil(t, outerCtx)
		assert.NotNil(t, outerSpan)

		return service.WithSpan(outerCtx, "inner-span", func(innerCtx context.Context, innerSpan trace.Span) error {
			innerExecuted = true
			assert.NotNil(t, innerCtx)
			assert.NotNil(t, innerSpan)
			assert.NotEqual(t, outerSpan, innerSpan)
			return nil
		})
	})

	assert.NoError(t, err)
	assert.True(t, outerExecuted)
	assert.True(t, innerExecuted)
}

func TestTracingService_ConcurrentSpans(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	done := make(chan bool, 10)

	// Start 10 concurrent spans
	for i := 0; i < 10; i++ {
		go func(id int) {
			err := service.WithSpan(ctx, "concurrent-span", func(ctx context.Context, span trace.Span) error {
				time.Sleep(10 * time.Millisecond)
				service.AddSpanAttributes(ctx, attribute.Int("span.id", id))
				return nil
			})
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all spans to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestTracingService_AttributeTypes(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()
	ctx, span := service.StartSpan(ctx, "attribute-test")
	defer span.End()

	// Test various attribute types
	service.AddSpanAttributes(ctx,
		attribute.String("string.attr", "test-value"),
		attribute.Int("int.attr", 42),
		attribute.Int64("int64.attr", 9223372036854775807),
		attribute.Float64("float64.attr", 3.14159),
		attribute.Bool("bool.attr", true),
		attribute.StringSlice("string_slice.attr", []string{"a", "b", "c"}),
		attribute.IntSlice("int_slice.attr", []int{1, 2, 3}),
		attribute.Float64Slice("float64_slice.attr", []float64{1.1, 2.2, 3.3}),
		attribute.BoolSlice("bool_slice.attr", []bool{true, false, true}),
	)

	// Should not panic with noop tracer
}

func TestTraceableFunction(t *testing.T) {
	logger := logrus.New()
	logger.SetOutput(io.Discard)

	config := &TracingConfig{
		ServiceName: "test-service",
		Enabled:     false,
	}

	service, err := NewTracingService(config, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Test TraceableFunction type
	var fn TraceableFunction = func(ctx context.Context, span trace.Span) error {
		assert.NotNil(t, ctx)
		assert.NotNil(t, span)
		return nil
	}

	err = service.TraceOperation(ctx, "traceable-function-test", nil, fn)
	assert.NoError(t, err)
}