package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/prometheus/client_golang/prometheus"
)

// RealTimeAnalyticsPipeline processes events in real-time for business intelligence
type RealTimeAnalyticsPipeline struct {
	kafkaConsumer    *kafka.Consumer
	kafkaProducer    *kafka.Producer
	clickhouseClient ClickHouseClient
	redisClient      RedisClient
	processors       map[string]EventProcessor
	metrics          *PipelineMetrics
	logger           *slog.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

// EventProcessor defines the interface for processing different event types
type EventProcessor interface {
	Process(ctx context.Context, event *AnalyticsEvent) (*ProcessedEvent, error)
	GetEventTypes() []string
}

// AnalyticsEvent represents an event in the analytics pipeline
type AnalyticsEvent struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Source        string                 `json:"source"`
	UserID        string                 `json:"user_id,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	HotelID       string                 `json:"hotel_id,omitempty"`
	ReviewID      string                 `json:"review_id,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	Properties    map[string]interface{} `json:"properties"`
	Metadata      map[string]interface{} `json:"metadata"`
	IPAddress     string                 `json:"ip_address,omitempty"`
	UserAgent     string                 `json:"user_agent,omitempty"`
	GeoLocation   *GeoLocation           `json:"geo_location,omitempty"`
	DeviceInfo    *DeviceInfo            `json:"device_info,omitempty"`
}

// ProcessedEvent represents the result of event processing
type ProcessedEvent struct {
	OriginalEvent *AnalyticsEvent        `json:"original_event"`
	Metrics       []Metric               `json:"metrics"`
	Aggregations  []Aggregation          `json:"aggregations"`
	Enrichments   map[string]interface{} `json:"enrichments"`
	Timestamp     time.Time              `json:"timestamp"`
}

// Metric represents a computed metric
type Metric struct {
	Name      string                 `json:"name"`
	Value     float64                `json:"value"`
	Unit      string                 `json:"unit"`
	Tags      map[string]string      `json:"tags"`
	Timestamp time.Time              `json:"timestamp"`
	TTL       time.Duration          `json:"ttl"`
}

// Aggregation represents an aggregated value
type Aggregation struct {
	Key       string    `json:"key"`
	Value     float64   `json:"value"`
	Count     int64     `json:"count"`
	Window    string    `json:"window"`
	Timestamp time.Time `json:"timestamp"`
}

// GeoLocation represents geographical information
type GeoLocation struct {
	Country   string  `json:"country"`
	Region    string  `json:"region"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
}

// DeviceInfo represents device information
type DeviceInfo struct {
	Type           string `json:"type"`
	OS             string `json:"os"`
	Browser        string `json:"browser"`
	ScreenWidth    int    `json:"screen_width"`
	ScreenHeight   int    `json:"screen_height"`
	IsMobile       bool   `json:"is_mobile"`
	IsTablet       bool   `json:"is_tablet"`
	IsDesktop      bool   `json:"is_desktop"`
}

// ClickHouseClient defines the interface for ClickHouse operations
type ClickHouseClient interface {
	Insert(ctx context.Context, table string, data interface{}) error
	Query(ctx context.Context, query string, dest interface{}) error
	BatchInsert(ctx context.Context, table string, data []interface{}) error
}

// RedisClient defines the interface for Redis operations
type RedisClient interface {
	Incr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)
	ZIncr(ctx context.Context, key string, member string, increment float64) (float64, error)
	HIncrBy(ctx context.Context, key string, field string, incr int64) (int64, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

// PipelineMetrics holds metrics for the analytics pipeline
type PipelineMetrics struct {
	EventsProcessed    prometheus.Counter
	EventsProcessedTotal prometheus.CounterVec
	ProcessingLatency  prometheus.Histogram
	ProcessingErrors   prometheus.Counter
	QueueDepth         prometheus.Gauge
	ThroughputRate     prometheus.Gauge
}

// NewRealTimeAnalyticsPipeline creates a new real-time analytics pipeline
func NewRealTimeAnalyticsPipeline(
	kafkaConfig *kafka.ConfigMap,
	clickhouseClient ClickHouseClient,
	redisClient RedisClient,
	logger *slog.Logger,
) (*RealTimeAnalyticsPipeline, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create Kafka consumer
	consumer, err := kafka.NewConsumer(kafkaConfig)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create Kafka consumer: %w", err)
	}

	// Create Kafka producer for processed events
	producer, err := kafka.NewProducer(kafkaConfig)
	if err != nil {
		cancel()
		consumer.Close()
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	pipeline := &RealTimeAnalyticsPipeline{
		kafkaConsumer:    consumer,
		kafkaProducer:    producer,
		clickhouseClient: clickhouseClient,
		redisClient:      redisClient,
		processors:       make(map[string]EventProcessor),
		metrics:          createPipelineMetrics(),
		logger:           logger,
		ctx:              ctx,
		cancel:           cancel,
	}

	// Register default processors
	pipeline.registerDefaultProcessors()

	return pipeline, nil
}

// RegisterProcessor registers an event processor for specific event types
func (p *RealTimeAnalyticsPipeline) RegisterProcessor(processor EventProcessor) {
	for _, eventType := range processor.GetEventTypes() {
		p.processors[eventType] = processor
		p.logger.Info("Registered processor for event type", "event_type", eventType)
	}
}

// Start begins processing events from the pipeline
func (p *RealTimeAnalyticsPipeline) Start() error {
	topics := []string{
		"user-events",
		"hotel-events",
		"review-events",
		"search-events",
		"booking-events",
		"system-events",
	}

	if err := p.kafkaConsumer.SubscribeTopics(topics, nil); err != nil {
		return fmt.Errorf("failed to subscribe to topics: %w", err)
	}

	// Start consumer goroutines
	for i := 0; i < 4; i++ { // 4 parallel consumers
		p.wg.Add(1)
		go p.consumeEvents()
	}

	// Start metrics reporter
	p.wg.Add(1)
	go p.reportMetrics()

	p.logger.Info("Real-time analytics pipeline started", "topics", topics)
	return nil
}

// Stop gracefully stops the analytics pipeline
func (p *RealTimeAnalyticsPipeline) Stop() error {
	p.logger.Info("Stopping real-time analytics pipeline...")
	
	p.cancel()
	p.wg.Wait()
	
	p.kafkaConsumer.Close()
	p.kafkaProducer.Close()
	
	p.logger.Info("Real-time analytics pipeline stopped")
	return nil
}

// consumeEvents processes events from Kafka
func (p *RealTimeAnalyticsPipeline) consumeEvents() {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			msg, err := p.kafkaConsumer.ReadMessage(1 * time.Second)
			if err != nil {
				if err.(kafka.Error).Code() != kafka.ErrTimedOut {
					p.logger.Error("Error reading message", "error", err)
					p.metrics.ProcessingErrors.Inc()
				}
				continue
			}

			p.processMessage(msg)
		}
	}
}

// processMessage processes a single Kafka message
func (p *RealTimeAnalyticsPipeline) processMessage(msg *kafka.Message) {
	start := time.Now()
	
	defer func() {
		p.metrics.ProcessingLatency.Observe(time.Since(start).Seconds())
		p.metrics.EventsProcessed.Inc()
	}()

	// Parse the analytics event
	var event AnalyticsEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		p.logger.Error("Failed to unmarshal event", "error", err, "message", string(msg.Value))
		p.metrics.ProcessingErrors.Inc()
		return
	}

	// Enrich event with additional information
	p.enrichEvent(&event)

	// Find appropriate processor
	processor, exists := p.processors[event.Type]
	if !exists {
		p.logger.Debug("No processor found for event type", "event_type", event.Type)
		return
	}

	// Process the event
	processedEvent, err := processor.Process(p.ctx, &event)
	if err != nil {
		p.logger.Error("Failed to process event", "error", err, "event_id", event.ID, "event_type", event.Type)
		p.metrics.ProcessingErrors.Inc()
		return
	}

	// Store processed event and metrics
	if err := p.storeProcessedEvent(processedEvent); err != nil {
		p.logger.Error("Failed to store processed event", "error", err, "event_id", event.ID)
		p.metrics.ProcessingErrors.Inc()
		return
	}

	// Update real-time metrics
	if err := p.updateRealTimeMetrics(processedEvent); err != nil {
		p.logger.Error("Failed to update real-time metrics", "error", err, "event_id", event.ID)
	}

	// Publish processed event for downstream consumers
	if err := p.publishProcessedEvent(processedEvent); err != nil {
		p.logger.Error("Failed to publish processed event", "error", err, "event_id", event.ID)
	}

	p.metrics.EventsProcessedTotal.WithLabelValues(event.Type, event.Source).Inc()
}

// enrichEvent adds additional context to the event
func (p *RealTimeAnalyticsPipeline) enrichEvent(event *AnalyticsEvent) {
	// Add server timestamp if not present
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Enrich with geo-location if IP address is available
	if event.IPAddress != "" && event.GeoLocation == nil {
		if geoLocation := p.getGeoLocation(event.IPAddress); geoLocation != nil {
			event.GeoLocation = geoLocation
		}
	}

	// Parse user agent for device information
	if event.UserAgent != "" && event.DeviceInfo == nil {
		if deviceInfo := p.parseUserAgent(event.UserAgent); deviceInfo != nil {
			event.DeviceInfo = deviceInfo
		}
	}

	// Add session information if not present
	if event.SessionID == "" && event.UserID != "" {
		if sessionID := p.getOrCreateSession(event.UserID, event.IPAddress); sessionID != "" {
			event.SessionID = sessionID
		}
	}
}

// storeProcessedEvent stores the processed event in ClickHouse
func (p *RealTimeAnalyticsPipeline) storeProcessedEvent(event *ProcessedEvent) error {
	// Store in events table
	if err := p.clickhouseClient.Insert(p.ctx, "events", event.OriginalEvent); err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	// Store metrics
	for _, metric := range event.Metrics {
		if err := p.clickhouseClient.Insert(p.ctx, "metrics", metric); err != nil {
			p.logger.Error("Failed to insert metric", "error", err, "metric", metric.Name)
		}
	}

	// Store aggregations
	for _, aggregation := range event.Aggregations {
		if err := p.clickhouseClient.Insert(p.ctx, "aggregations", aggregation); err != nil {
			p.logger.Error("Failed to insert aggregation", "error", err, "key", aggregation.Key)
		}
	}

	return nil
}

// updateRealTimeMetrics updates Redis-based real-time metrics
func (p *RealTimeAnalyticsPipeline) updateRealTimeMetrics(event *ProcessedEvent) error {
	originalEvent := event.OriginalEvent
	timestamp := originalEvent.Timestamp

	// Update counters with time-based keys for different time windows
	hourKey := fmt.Sprintf("metrics:hour:%s", timestamp.Format("2006010215"))
	dayKey := fmt.Sprintf("metrics:day:%s", timestamp.Format("20060102"))
	weekKey := fmt.Sprintf("metrics:week:%s", timestamp.Format("2006-W01"))

	// Event type counters
	eventTypeKey := fmt.Sprintf("event_type:%s", originalEvent.Type)
	
	// Update hourly metrics
	p.redisClient.HIncrBy(p.ctx, hourKey, eventTypeKey, 1)
	p.redisClient.Expire(p.ctx, hourKey, 25*time.Hour) // Keep for 25 hours
	
	// Update daily metrics
	p.redisClient.HIncrBy(p.ctx, dayKey, eventTypeKey, 1)
	p.redisClient.Expire(p.ctx, dayKey, 8*24*time.Hour) // Keep for 8 days
	
	// Update weekly metrics
	p.redisClient.HIncrBy(p.ctx, weekKey, eventTypeKey, 1)
	p.redisClient.Expire(p.ctx, weekKey, 5*7*24*time.Hour) // Keep for 5 weeks

	// Update specific metrics based on event type
	switch originalEvent.Type {
	case "hotel_view":
		if originalEvent.HotelID != "" {
			// Popular hotels tracking
			p.redisClient.ZIncr(p.ctx, "popular_hotels:daily", originalEvent.HotelID, 1)
			p.redisClient.Expire(p.ctx, "popular_hotels:daily", 24*time.Hour)
		}
	case "search_performed":
		// Search metrics
		if query, ok := originalEvent.Properties["query"].(string); ok && query != "" {
			p.redisClient.ZIncr(p.ctx, "popular_searches:daily", query, 1)
			p.redisClient.Expire(p.ctx, "popular_searches:daily", 24*time.Hour)
		}
	case "review_created":
		if originalEvent.HotelID != "" {
			// Review activity tracking
			p.redisClient.HIncrBy(p.ctx, "hotel_reviews:daily", originalEvent.HotelID, 1)
			p.redisClient.Expire(p.ctx, "hotel_reviews:daily", 24*time.Hour)
		}
	}

	// User activity tracking
	if originalEvent.UserID != "" {
		userActivityKey := fmt.Sprintf("user_activity:%s:%s", originalEvent.UserID, timestamp.Format("20060102"))
		p.redisClient.HIncrBy(p.ctx, userActivityKey, originalEvent.Type, 1)
		p.redisClient.Expire(p.ctx, userActivityKey, 30*24*time.Hour) // Keep for 30 days
	}

	return nil
}

// publishProcessedEvent publishes the processed event to downstream topics
func (p *RealTimeAnalyticsPipeline) publishProcessedEvent(event *ProcessedEvent) error {
	// Marshal processed event
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal processed event: %w", err)
	}

	// Determine output topic based on event type
	topic := p.getOutputTopic(event.OriginalEvent.Type)

	// Produce message
	message := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(event.OriginalEvent.ID),
		Value: data,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.OriginalEvent.Type)},
			{Key: "source", Value: []byte(event.OriginalEvent.Source)},
			{Key: "processed_at", Value: []byte(time.Now().Format(time.RFC3339))},
		},
	}

	return p.kafkaProducer.Produce(message, nil)
}

// getOutputTopic determines the output topic for processed events
func (p *RealTimeAnalyticsPipeline) getOutputTopic(eventType string) string {
	switch eventType {
	case "hotel_view", "hotel_search", "hotel_booking":
		return "hotel-analytics"
	case "review_created", "review_updated", "review_moderated":
		return "review-analytics"
	case "user_login", "user_signup", "user_activity":
		return "user-analytics"
	case "search_performed", "search_results_clicked":
		return "search-analytics"
	default:
		return "general-analytics"
	}
}

// reportMetrics periodically reports pipeline metrics
func (p *RealTimeAnalyticsPipeline) reportMetrics() {
	defer p.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.updatePipelineMetrics()
		}
	}
}

// updatePipelineMetrics updates Prometheus metrics
func (p *RealTimeAnalyticsPipeline) updatePipelineMetrics() {
	// Update queue depth (approximate based on Kafka lag)
	// This would typically come from Kafka consumer metrics
	p.metrics.QueueDepth.Set(0) // Placeholder

	// Update throughput rate
	// This would be calculated based on recent processing rate
	p.metrics.ThroughputRate.Set(0) // Placeholder
}

// Helper methods for event enrichment

func (p *RealTimeAnalyticsPipeline) getGeoLocation(ipAddress string) *GeoLocation {
	// Placeholder implementation - would use a geo-IP service
	return &GeoLocation{
		Country:  "US",
		Region:   "CA",
		City:     "San Francisco",
		Timezone: "America/Los_Angeles",
	}
}

func (p *RealTimeAnalyticsPipeline) parseUserAgent(userAgent string) *DeviceInfo {
	// Placeholder implementation - would use a user agent parser
	return &DeviceInfo{
		Type:      "desktop",
		OS:        "Unknown",
		Browser:   "Unknown",
		IsDesktop: true,
	}
}

func (p *RealTimeAnalyticsPipeline) getOrCreateSession(userID, ipAddress string) string {
	// Placeholder implementation - would create or retrieve session ID
	return fmt.Sprintf("session_%s_%d", userID, time.Now().Unix())
}

// registerDefaultProcessors registers the default event processors
func (p *RealTimeAnalyticsPipeline) registerDefaultProcessors() {
	p.RegisterProcessor(NewHotelEventProcessor(p.redisClient, p.logger))
	p.RegisterProcessor(NewReviewEventProcessor(p.redisClient, p.logger))
	p.RegisterProcessor(NewUserEventProcessor(p.redisClient, p.logger))
	p.RegisterProcessor(NewSearchEventProcessor(p.redisClient, p.logger))
}

// createPipelineMetrics creates Prometheus metrics for the pipeline
func createPipelineMetrics() *PipelineMetrics {
	return &PipelineMetrics{
		EventsProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "analytics_events_processed_total",
			Help: "Total number of events processed",
		}),
		EventsProcessedTotal: *prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "analytics_events_processed_by_type_total",
			Help: "Total number of events processed by type and source",
		}, []string{"event_type", "source"}),
		ProcessingLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytics_processing_duration_seconds",
			Help:    "Event processing latency",
			Buckets: prometheus.DefBuckets,
		}),
		ProcessingErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "analytics_processing_errors_total",
			Help: "Total number of processing errors",
		}),
		QueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_queue_depth",
			Help: "Current queue depth",
		}),
		ThroughputRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "analytics_throughput_rate",
			Help: "Current throughput rate (events/second)",
		}),
	}
}