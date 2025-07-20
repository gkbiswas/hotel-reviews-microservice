package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// Test helper to create a test async processor without real Kafka
func createTestAsyncProcessor(workerCount int) *AsyncProcessor {
	// Create with nil Kafka components for unit testing
	log := logger.NewDefault()
	return NewAsyncProcessor(nil, nil, log, workerCount)
}

func TestNewAsyncProcessor(t *testing.T) {
	t.Run("create new async processor", func(t *testing.T) {
		log := logger.NewDefault()
		workerCount := 5

		processor := NewAsyncProcessor(nil, nil, log, workerCount)

		assert.NotNil(t, processor)
		assert.Equal(t, log, processor.logger)
		assert.Equal(t, workerCount, processor.workerCount)
		assert.NotNil(t, processor.handlers)
		assert.NotNil(t, processor.ctx)
		assert.NotNil(t, processor.cancel)
	})

	t.Run("handlers map is initialized", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		assert.NotNil(t, processor.handlers)
		assert.Len(t, processor.handlers, 0)
	})
}

func TestAsyncProcessor_RegisterHandler(t *testing.T) {
	t.Run("register single handler", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		handler := func(ctx context.Context, message []byte) error {
			return nil
		}

		processor.RegisterHandler("test-message", handler)

		processor.mu.RLock()
		registeredHandler, exists := processor.handlers["test-message"]
		processor.mu.RUnlock()

		assert.True(t, exists)
		assert.NotNil(t, registeredHandler)
	})

	t.Run("register multiple handlers", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		handler1 := func(ctx context.Context, message []byte) error { return nil }
		handler2 := func(ctx context.Context, message []byte) error { return nil }
		handler3 := func(ctx context.Context, message []byte) error { return nil }

		processor.RegisterHandler("message-type-1", handler1)
		processor.RegisterHandler("message-type-2", handler2)
		processor.RegisterHandler("message-type-3", handler3)

		processor.mu.RLock()
		assert.Len(t, processor.handlers, 3)
		assert.Contains(t, processor.handlers, "message-type-1")
		assert.Contains(t, processor.handlers, "message-type-2")
		assert.Contains(t, processor.handlers, "message-type-3")
		processor.mu.RUnlock()
	})

	t.Run("override existing handler", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		handler1 := func(ctx context.Context, message []byte) error { return nil }
		handler2 := func(ctx context.Context, message []byte) error { return nil }

		processor.RegisterHandler("test-message", handler1)
		processor.RegisterHandler("test-message", handler2)

		processor.mu.RLock()
		assert.Len(t, processor.handlers, 1)
		assert.Contains(t, processor.handlers, "test-message")
		processor.mu.RUnlock()
	})

	t.Run("concurrent handler registration", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		const numGoroutines = 10
		var wg sync.WaitGroup

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				handler := func(ctx context.Context, message []byte) error { return nil }
				processor.RegisterHandler(fmt.Sprintf("handler-%d", id), handler)
			}(i)
		}

		wg.Wait()

		processor.mu.RLock()
		assert.Len(t, processor.handlers, numGoroutines)
		processor.mu.RUnlock()
	})
}

func TestAsyncProcessor_StartStop(t *testing.T) {
	t.Run("stop processor without starting", func(t *testing.T) {
		processor := createTestAsyncProcessor(2)

		// Stop the processor without starting (should handle gracefully)
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := processor.Stop(stopCtx)
		assert.NoError(t, err)
	})

	t.Run("stop with short timeout", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		// Try to stop with short timeout when no workers are running
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		err := processor.Stop(stopCtx)
		// Should complete quickly since no workers are running
		assert.NoError(t, err)
	})

	t.Run("multiple stop calls", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		// Multiple stop calls should not panic
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err1 := processor.Stop(stopCtx)
		err2 := processor.Stop(stopCtx)
		err3 := processor.Stop(stopCtx)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.NoError(t, err3)
	})
}

func TestAsyncProcessor_ErrorHandling(t *testing.T) {
	t.Run("handler returns error", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		// Register a handler that returns an error
		expectedError := assert.AnError
		handler := func(ctx context.Context, message []byte) error {
			return expectedError
		}

		processor.RegisterHandler("error-message", handler)

		// This test verifies the handler interface works
		// The actual error handling would be tested in integration tests
		// since it depends on the worker goroutine implementation
		assert.NotNil(t, processor.handlers["error-message"])
	})

	t.Run("missing handler for message type", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		// No handlers registered
		processor.mu.RLock()
		_, exists := processor.handlers["unknown-message"]
		processor.mu.RUnlock()

		assert.False(t, exists)
	})
}

func TestAsyncProcessor_ContextCancellation(t *testing.T) {
	t.Run("processor context can be cancelled", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		// Verify the processor has a cancellable context
		assert.NotNil(t, processor.ctx)
		assert.NotNil(t, processor.cancel)

		// Cancel the context
		processor.cancel()

		// Verify context is cancelled
		select {
		case <-processor.ctx.Done():
			assert.Equal(t, context.Canceled, processor.ctx.Err())
		default:
			t.Fatal("Context was not cancelled")
		}
	})
}

func TestMessageHandler_Interface(t *testing.T) {
	t.Run("message handler interface", func(t *testing.T) {
		// Test that MessageHandler is properly defined
		var handler MessageHandler

		// Should be nil initially
		assert.Nil(t, handler)

		// Create a handler implementation
		handler = func(ctx context.Context, message []byte) error {
			return nil
		}

		assert.NotNil(t, handler)

		// Test handler execution
		err := handler(context.Background(), []byte("test message"))
		assert.NoError(t, err)
	})

	t.Run("handler with error", func(t *testing.T) {
		handler := MessageHandler(func(ctx context.Context, message []byte) error {
			return assert.AnError
		})

		err := handler(context.Background(), []byte("test"))
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})

	t.Run("handler with context cancellation", func(t *testing.T) {
		handler := MessageHandler(func(ctx context.Context, message []byte) error {
			return ctx.Err()
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := handler(ctx, []byte("test"))
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})
}

func TestAsyncProcessor_ConcurrentOperations(t *testing.T) {
	t.Run("concurrent handler registration", func(t *testing.T) {
		processor := createTestAsyncProcessor(1)

		var wg sync.WaitGroup

		// Register handlers concurrently
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				handler := func(ctx context.Context, message []byte) error {
					return nil
				}
				processor.RegisterHandler(fmt.Sprintf("handler-%d", id), handler)
			}(i)
		}

		wg.Wait()

		// Verify state
		processor.mu.RLock()
		assert.Len(t, processor.handlers, 5)
		processor.mu.RUnlock()
	})
}

func TestAsyncProcessor_WorkerCount(t *testing.T) {
	t.Run("different worker counts", func(t *testing.T) {
		tests := []int{1, 5, 10, 20}

		for _, workerCount := range tests {
			t.Run(fmt.Sprintf("worker_count_%d", workerCount), func(t *testing.T) {
				processor := createTestAsyncProcessor(workerCount)
				assert.Equal(t, workerCount, processor.workerCount)

				// Test that processor structure is correctly initialized
				assert.NotNil(t, processor.handlers)
				assert.NotNil(t, processor.ctx)
				assert.NotNil(t, processor.cancel)
			})
		}
	})
}

func TestAsyncProcessor_Properties(t *testing.T) {
	t.Run("processor properties", func(t *testing.T) {
		log := logger.NewDefault()
		processor := NewAsyncProcessor(nil, nil, log, 3)

		assert.Equal(t, 3, processor.workerCount)
		assert.Equal(t, log, processor.logger)
		assert.NotNil(t, processor.ctx)
		assert.NotNil(t, processor.cancel)
		assert.NotNil(t, processor.handlers)
		assert.Len(t, processor.handlers, 0)
	})
}