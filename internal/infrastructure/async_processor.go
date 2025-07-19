package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

// AsyncProcessor handles asynchronous message processing
type AsyncProcessor struct {
	writer      *kafka.Writer
	reader      *kafka.Reader
	logger      interface{}
	workerCount int
	handlers    map[string]MessageHandler
	mu          sync.RWMutex
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

// MessageHandler processes messages of a specific type
type MessageHandler func(ctx context.Context, message []byte) error

// NewAsyncProcessor creates a new async processor
func NewAsyncProcessor(writer *kafka.Writer, reader *kafka.Reader, logger interface{}, workerCount int) *AsyncProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	return &AsyncProcessor{
		writer:      writer,
		reader:      reader,
		logger:      logger,
		workerCount: workerCount,
		handlers:    make(map[string]MessageHandler),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// RegisterHandler registers a handler for a message type
func (p *AsyncProcessor) RegisterHandler(messageType string, handler MessageHandler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[messageType] = handler
}

// Start starts the async processor workers
func (p *AsyncProcessor) Start(ctx context.Context) {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the async processor
func (p *AsyncProcessor) Stop(ctx context.Context) error {
	p.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PublishMessage publishes a message to Kafka
func (p *AsyncProcessor) PublishMessage(ctx context.Context, messageType string, data interface{}) error {
	payload, err := json.Marshal(map[string]interface{}{
		"type":      messageType,
		"data":      data,
		"timestamp": time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	message := kafka.Message{
		Key:   []byte(messageType),
		Value: payload,
	}

	err = p.writer.WriteMessages(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// worker processes messages from Kafka
func (p *AsyncProcessor) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			message, err := p.reader.ReadMessage(p.ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				// Log error and continue
				continue
			}

			// Process message
			p.processMessage(message)
		}
	}
}

// processMessage processes a single message
func (p *AsyncProcessor) processMessage(message kafka.Message) {
	var payload map[string]interface{}
	if err := json.Unmarshal(message.Value, &payload); err != nil {
		// Log error
		return
	}

	messageType, ok := payload["type"].(string)
	if !ok {
		// Log error
		return
	}

	p.mu.RLock()
	handler, exists := p.handlers[messageType]
	p.mu.RUnlock()

	if !exists {
		// Log unknown message type
		return
	}

	// Execute handler
	if err := handler(p.ctx, message.Value); err != nil {
		// Log error
		// In production, you might want to retry or send to DLQ
	}
}

// EnqueueReviewProcessing enqueues a review for processing
func (p *AsyncProcessor) EnqueueReviewProcessing(ctx context.Context, reviewID string) error {
	return p.PublishMessage(ctx, "review.process", map[string]interface{}{
		"review_id": reviewID,
		"action":    "process",
	})
}

// EnqueueEmailNotification enqueues an email notification
func (p *AsyncProcessor) EnqueueEmailNotification(ctx context.Context, to, subject, body string) error {
	return p.PublishMessage(ctx, "notification.email", map[string]interface{}{
		"to":      to,
		"subject": subject,
		"body":    body,
	})
}

// EnqueueImageProcessing enqueues image processing
func (p *AsyncProcessor) EnqueueImageProcessing(ctx context.Context, imageURL string) error {
	return p.PublishMessage(ctx, "image.process", map[string]interface{}{
		"image_url":  imageURL,
		"operations": []string{"resize", "optimize"},
	})
}
