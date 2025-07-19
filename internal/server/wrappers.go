package server

import (
	"context"
	
	"github.com/segmentio/kafka-go"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/infrastructure"
)

// KafkaWriterWrapper wraps Kafka writer for graceful shutdown
type KafkaWriterWrapper struct {
	Writer *kafka.Writer
}

func (w *KafkaWriterWrapper) Stop(ctx context.Context) error {
	return w.Writer.Close()
}

func (w *KafkaWriterWrapper) Name() string {
	return "kafka_writer"
}

// KafkaReaderWrapper wraps Kafka reader for graceful shutdown
type KafkaReaderWrapper struct {
	Reader *kafka.Reader
}

func (r *KafkaReaderWrapper) Stop(ctx context.Context) error {
	return r.Reader.Close()
}

func (r *KafkaReaderWrapper) Name() string {
	return "kafka_reader"
}

// AsyncProcessorWrapper wraps async processor for graceful shutdown
type AsyncProcessorWrapper struct {
	Processor *infrastructure.AsyncProcessor
}

func (a *AsyncProcessorWrapper) Stop(ctx context.Context) error {
	if a.Processor == nil {
		return nil
	}
	return a.Processor.Stop(ctx)
}

func (a *AsyncProcessorWrapper) Name() string {
	return "async_processor"
}