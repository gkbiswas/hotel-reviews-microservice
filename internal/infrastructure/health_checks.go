package infrastructure

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// NewRedisHealthCheck creates a health check for Redis
func NewRedisHealthCheck(client *redis.Client) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := client.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis health check failed: %w", err)
		}
		return nil
	}
}

// NewKafkaHealthCheck creates a health check for Kafka
func NewKafkaHealthCheck(writer *kafka.Writer) func(context.Context) error {
	return func(ctx context.Context) error {
		// For Kafka, we'll just check if the writer is configured
		// In production, you might want to actually try to connect to the broker
		if writer == nil {
			return fmt.Errorf("kafka writer is not configured")
		}
		return nil
	}
}
