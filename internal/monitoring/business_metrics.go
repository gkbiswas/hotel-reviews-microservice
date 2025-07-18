package monitoring

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
)

// BusinessMetrics tracks hotel reviews specific business metrics
type BusinessMetrics struct {
	// Review processing metrics
	ReviewsIngested            *prometheus.CounterVec
	ReviewsValidated           *prometheus.CounterVec
	ReviewsStored              *prometheus.CounterVec
	ReviewsRejected            *prometheus.CounterVec
	ReviewsDeduplicationHits   *prometheus.CounterVec
	
	// File processing metrics
	FilesProcessed             *prometheus.CounterVec
	FileProcessingTime         *prometheus.HistogramVec
	FileSize                   *prometheus.HistogramVec
	RecordsPerFile             *prometheus.HistogramVec
	
	// Provider metrics
	ProviderRequestsTotal      *prometheus.CounterVec
	ProviderResponseTime       *prometheus.HistogramVec
	ProviderErrors             *prometheus.CounterVec
	ProviderDataQuality        *prometheus.GaugeVec
	
	// Hotel metrics
	HotelsTotal                *prometheus.GaugeVec
	HotelsWithReviews          *prometheus.GaugeVec
	AverageRatingByHotel       *prometheus.GaugeVec
	ReviewCountByHotel         *prometheus.GaugeVec
	
	// Data quality metrics
	DataQualityScore           *prometheus.GaugeVec
	MissingFieldsCount         *prometheus.CounterVec
	InvalidDataCount           *prometheus.CounterVec
	DataCompleteness           *prometheus.GaugeVec
	
	// User engagement metrics
	ReviewsPerUser             *prometheus.HistogramVec
	UserActivityPattern       *prometheus.CounterVec
	ReviewSentimentDistribution *prometheus.CounterVec
	
	// Business KPIs
	ReviewVelocity             *prometheus.GaugeVec
	ReviewBacklog              *prometheus.GaugeVec
	DataFreshness              *prometheus.GaugeVec
	CustomerSatisfactionScore  *prometheus.GaugeVec
	
	// Operational metrics
	ProcessingCapacity         *prometheus.GaugeVec
	ProcessingUtilization      *prometheus.GaugeVec
	QueueDepth                 *prometheus.GaugeVec
	ErrorRate                  *prometheus.GaugeVec
	
	logger *logrus.Logger
}

// NewBusinessMetrics creates a new business metrics collector
func NewBusinessMetrics(logger *logrus.Logger) *BusinessMetrics {
	return &BusinessMetrics{
		// Review processing metrics
		ReviewsIngested: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_ingested_total",
				Help: "Total number of reviews ingested",
			},
			[]string{"provider", "hotel_id", "source"},
		),
		ReviewsValidated: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_validated_total",
				Help: "Total number of reviews validated",
			},
			[]string{"provider", "validation_status"},
		),
		ReviewsStored: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_stored_total",
				Help: "Total number of reviews stored successfully",
			},
			[]string{"provider", "hotel_id"},
		),
		ReviewsRejected: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_rejected_total",
				Help: "Total number of reviews rejected",
			},
			[]string{"provider", "rejection_reason"},
		),
		ReviewsDeduplicationHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_deduplication_hits_total",
				Help: "Total number of duplicate reviews detected",
			},
			[]string{"provider", "deduplication_type"},
		),
		
		// File processing metrics
		FilesProcessed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_files_processed_total",
				Help: "Total number of files processed",
			},
			[]string{"provider", "file_type", "status"},
		),
		FileProcessingTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_file_processing_duration_seconds",
				Help:    "Time taken to process a file",
				Buckets: prometheus.ExponentialBuckets(1, 2, 12), // 1s to ~1h
			},
			[]string{"provider", "file_type"},
		),
		FileSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_file_size_bytes",
				Help:    "Size of processed files in bytes",
				Buckets: prometheus.ExponentialBuckets(1024, 10, 8), // 1KB to ~1GB
			},
			[]string{"provider", "file_type"},
		),
		RecordsPerFile: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_records_per_file",
				Help:    "Number of records in each processed file",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6), // 100 to 100M
			},
			[]string{"provider", "file_type"},
		),
		
		// Provider metrics
		ProviderRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_provider_requests_total",
				Help: "Total number of requests to providers",
			},
			[]string{"provider", "operation", "status"},
		),
		ProviderResponseTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_provider_response_time_seconds",
				Help:    "Response time for provider requests",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"provider", "operation"},
		),
		ProviderErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_provider_errors_total",
				Help: "Total number of provider errors",
			},
			[]string{"provider", "error_type"},
		),
		ProviderDataQuality: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_provider_data_quality_score",
				Help: "Data quality score for each provider (0-1)",
			},
			[]string{"provider", "metric_type"},
		),
		
		// Hotel metrics
		HotelsTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_hotels_total",
				Help: "Total number of hotels in the system",
			},
			[]string{"provider", "location"},
		),
		HotelsWithReviews: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_hotels_with_reviews",
				Help: "Number of hotels that have reviews",
			},
			[]string{"provider", "location"},
		),
		AverageRatingByHotel: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_average_rating_by_hotel",
				Help: "Average rating for each hotel",
			},
			[]string{"provider", "hotel_id"},
		),
		ReviewCountByHotel: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_count_by_hotel",
				Help: "Number of reviews for each hotel",
			},
			[]string{"provider", "hotel_id"},
		),
		
		// Data quality metrics
		DataQualityScore: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_data_quality_score",
				Help: "Overall data quality score (0-1)",
			},
			[]string{"provider", "dimension"},
		),
		MissingFieldsCount: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_missing_fields_total",
				Help: "Total number of missing fields in reviews",
			},
			[]string{"provider", "field_name"},
		),
		InvalidDataCount: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_invalid_data_total",
				Help: "Total number of invalid data entries",
			},
			[]string{"provider", "field_name", "validation_rule"},
		),
		DataCompleteness: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_data_completeness",
				Help: "Data completeness percentage for each field",
			},
			[]string{"provider", "field_name"},
		),
		
		// User engagement metrics
		ReviewsPerUser: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "hotel_reviews_per_user",
				Help:    "Number of reviews per user",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10),
			},
			[]string{"provider"},
		),
		UserActivityPattern: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_user_activity_pattern",
				Help: "User activity patterns",
			},
			[]string{"provider", "time_period", "activity_type"},
		),
		ReviewSentimentDistribution: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "hotel_reviews_sentiment_distribution",
				Help: "Distribution of review sentiments",
			},
			[]string{"provider", "sentiment", "rating_range"},
		),
		
		// Business KPIs
		ReviewVelocity: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_velocity_per_hour",
				Help: "Number of reviews processed per hour",
			},
			[]string{"provider"},
		),
		ReviewBacklog: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_backlog_count",
				Help: "Number of reviews waiting to be processed",
			},
			[]string{"provider", "priority"},
		),
		DataFreshness: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_data_freshness_hours",
				Help: "Age of the latest data in hours",
			},
			[]string{"provider", "data_type"},
		),
		CustomerSatisfactionScore: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_customer_satisfaction_score",
				Help: "Customer satisfaction score based on reviews",
			},
			[]string{"provider", "hotel_id", "time_period"},
		),
		
		// Operational metrics
		ProcessingCapacity: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_processing_capacity",
				Help: "Maximum processing capacity",
			},
			[]string{"component", "resource_type"},
		),
		ProcessingUtilization: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_processing_utilization",
				Help: "Current processing utilization percentage",
			},
			[]string{"component", "resource_type"},
		),
		QueueDepth: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_queue_depth",
				Help: "Current queue depth",
			},
			[]string{"queue_type", "priority"},
		),
		ErrorRate: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "hotel_reviews_error_rate",
				Help: "Error rate percentage",
			},
			[]string{"component", "error_type"},
		),
		
		logger: logger,
	}
}

// RecordReviewIngestion records review ingestion metrics
func (m *BusinessMetrics) RecordReviewIngestion(provider, hotelID, source string) {
	m.ReviewsIngested.WithLabelValues(provider, hotelID, source).Inc()
}

// RecordReviewValidation records review validation metrics
func (m *BusinessMetrics) RecordReviewValidation(provider, validationStatus string) {
	m.ReviewsValidated.WithLabelValues(provider, validationStatus).Inc()
}

// RecordReviewStorage records review storage metrics
func (m *BusinessMetrics) RecordReviewStorage(provider, hotelID string) {
	m.ReviewsStored.WithLabelValues(provider, hotelID).Inc()
}

// RecordReviewRejection records review rejection metrics
func (m *BusinessMetrics) RecordReviewRejection(provider, rejectionReason string) {
	m.ReviewsRejected.WithLabelValues(provider, rejectionReason).Inc()
}

// RecordFileProcessing records file processing metrics
func (m *BusinessMetrics) RecordFileProcessing(provider, fileType, status string, duration time.Duration, fileSize int64, recordCount int) {
	m.FilesProcessed.WithLabelValues(provider, fileType, status).Inc()
	m.FileProcessingTime.WithLabelValues(provider, fileType).Observe(duration.Seconds())
	m.FileSize.WithLabelValues(provider, fileType).Observe(float64(fileSize))
	m.RecordsPerFile.WithLabelValues(provider, fileType).Observe(float64(recordCount))
}

// RecordProviderRequest records provider request metrics
func (m *BusinessMetrics) RecordProviderRequest(provider, operation, status string, duration time.Duration) {
	m.ProviderRequestsTotal.WithLabelValues(provider, operation, status).Inc()
	m.ProviderResponseTime.WithLabelValues(provider, operation).Observe(duration.Seconds())
}

// RecordProviderError records provider error metrics
func (m *BusinessMetrics) RecordProviderError(provider, errorType string) {
	m.ProviderErrors.WithLabelValues(provider, errorType).Inc()
}

// UpdateDataQuality updates data quality metrics
func (m *BusinessMetrics) UpdateDataQuality(provider, dimension string, score float64) {
	m.DataQualityScore.WithLabelValues(provider, dimension).Set(score)
}

// RecordMissingField records missing field metrics
func (m *BusinessMetrics) RecordMissingField(provider, fieldName string) {
	m.MissingFieldsCount.WithLabelValues(provider, fieldName).Inc()
}

// RecordInvalidData records invalid data metrics
func (m *BusinessMetrics) RecordInvalidData(provider, fieldName, validationRule string) {
	m.InvalidDataCount.WithLabelValues(provider, fieldName, validationRule).Inc()
}

// UpdateDataCompleteness updates data completeness metrics
func (m *BusinessMetrics) UpdateDataCompleteness(provider, fieldName string, completeness float64) {
	m.DataCompleteness.WithLabelValues(provider, fieldName).Set(completeness)
}

// UpdateHotelMetrics updates hotel-related metrics
func (m *BusinessMetrics) UpdateHotelMetrics(provider, hotelID, location string, totalHotels, hotelsWithReviews int, avgRating float64, reviewCount int) {
	m.HotelsTotal.WithLabelValues(provider, location).Set(float64(totalHotels))
	m.HotelsWithReviews.WithLabelValues(provider, location).Set(float64(hotelsWithReviews))
	m.AverageRatingByHotel.WithLabelValues(provider, hotelID).Set(avgRating)
	m.ReviewCountByHotel.WithLabelValues(provider, hotelID).Set(float64(reviewCount))
}

// RecordUserActivity records user activity metrics
func (m *BusinessMetrics) RecordUserActivity(provider, timePeriod, activityType string) {
	m.UserActivityPattern.WithLabelValues(provider, timePeriod, activityType).Inc()
}

// RecordReviewSentiment records review sentiment metrics
func (m *BusinessMetrics) RecordReviewSentiment(provider, sentiment, ratingRange string) {
	m.ReviewSentimentDistribution.WithLabelValues(provider, sentiment, ratingRange).Inc()
}

// UpdateBusinessKPIs updates business KPI metrics
func (m *BusinessMetrics) UpdateBusinessKPIs(provider, hotelID, timePeriod string, velocity, backlog, freshness, satisfaction float64) {
	m.ReviewVelocity.WithLabelValues(provider).Set(velocity)
	m.ReviewBacklog.WithLabelValues(provider, "normal").Set(backlog)
	m.DataFreshness.WithLabelValues(provider, "reviews").Set(freshness)
	m.CustomerSatisfactionScore.WithLabelValues(provider, hotelID, timePeriod).Set(satisfaction)
}

// UpdateOperationalMetrics updates operational metrics
func (m *BusinessMetrics) UpdateOperationalMetrics(component, resourceType string, capacity, utilization float64) {
	m.ProcessingCapacity.WithLabelValues(component, resourceType).Set(capacity)
	m.ProcessingUtilization.WithLabelValues(component, resourceType).Set(utilization)
}

// UpdateQueueDepth updates queue depth metrics
func (m *BusinessMetrics) UpdateQueueDepth(queueType, priority string, depth float64) {
	m.QueueDepth.WithLabelValues(queueType, priority).Set(depth)
}

// UpdateErrorRate updates error rate metrics
func (m *BusinessMetrics) UpdateErrorRate(component, errorType string, rate float64) {
	m.ErrorRate.WithLabelValues(component, errorType).Set(rate)
}

// StartBusinessMetricsCollection starts periodic collection of business metrics
func (m *BusinessMetrics) StartBusinessMetricsCollection(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.collectBusinessMetrics(ctx)
			}
		}
	}()
}

// collectBusinessMetrics collects business metrics from the system
func (m *BusinessMetrics) collectBusinessMetrics(ctx context.Context) {
	// This would typically query the database and other systems
	// to collect current business metrics
	
	// For demonstration, we'll update some sample metrics
	m.logger.Debug("Collecting business metrics")
	
	// Update some sample operational metrics
	m.UpdateOperationalMetrics("processing_engine", "cpu", 100.0, 75.0)
	m.UpdateOperationalMetrics("processing_engine", "memory", 8192.0, 6144.0)
	m.UpdateQueueDepth("file_processing", "normal", 50.0)
	m.UpdateQueueDepth("file_processing", "high", 10.0)
	
	// Update some sample data quality metrics
	m.UpdateDataQuality("booking", "completeness", 0.95)
	m.UpdateDataQuality("booking", "accuracy", 0.92)
	m.UpdateDataQuality("expedia", "completeness", 0.88)
	m.UpdateDataQuality("expedia", "accuracy", 0.90)
}