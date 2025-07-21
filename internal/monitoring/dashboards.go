package monitoring

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// DashboardConfig represents a monitoring dashboard configuration
type DashboardConfig struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Panels      []DashboardPanel       `json:"panels"`
	Filters     []DashboardFilter      `json:"filters"`
	RefreshRate time.Duration          `json:"refresh_rate"`
	Tags        []string               `json:"tags"`
	Variables   map[string]interface{} `json:"variables"`
}

// DashboardPanel represents a panel in a dashboard
type DashboardPanel struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Type        string                 `json:"type"` // graph, table, stat, gauge, etc.
	Query       string                 `json:"query"`
	Unit        string                 `json:"unit"`
	Thresholds  []Threshold            `json:"thresholds"`
	Position    PanelPosition          `json:"position"`
	Options     map[string]interface{} `json:"options"`
}

// PanelPosition defines the position and size of a panel
type PanelPosition struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Threshold defines alert thresholds for panels
type Threshold struct {
	Value float64 `json:"value"`
	Color string  `json:"color"`
	State string  `json:"state"` // ok, warning, critical
}

// DashboardFilter represents a filter for dashboard data
type DashboardFilter struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"` // dropdown, text, date, etc.
	Options  []string `json:"options"`
	Default  string   `json:"default"`
	Required bool     `json:"required"`
}

// DashboardManager manages monitoring dashboards
type DashboardManager struct {
	dashboards map[string]*DashboardConfig
	logger     *slog.Logger
}

// NewDashboardManager creates a new dashboard manager
func NewDashboardManager(logger *slog.Logger) *DashboardManager {
	dm := &DashboardManager{
		dashboards: make(map[string]*DashboardConfig),
		logger:     logger,
	}
	
	// Initialize default dashboards
	dm.initializeDefaultDashboards()
	
	return dm
}

// initializeDefaultDashboards creates the default monitoring dashboards
func (dm *DashboardManager) initializeDefaultDashboards() {
	// Business Overview Dashboard
	dm.AddDashboard(dm.createBusinessOverviewDashboard())
	
	// Performance Monitoring Dashboard
	dm.AddDashboard(dm.createPerformanceMonitoringDashboard())
	
	// Data Quality Dashboard
	dm.AddDashboard(dm.createDataQualityDashboard())
	
	// Operational Health Dashboard
	dm.AddDashboard(dm.createOperationalHealthDashboard())
	
	// Customer Insights Dashboard
	dm.AddDashboard(dm.createCustomerInsightsDashboard())
}

// createBusinessOverviewDashboard creates the main business metrics dashboard
func (dm *DashboardManager) createBusinessOverviewDashboard() *DashboardConfig {
	return &DashboardConfig{
		ID:          "business-overview",
		Title:       "Hotel Reviews - Business Overview",
		Description: "High-level business metrics and KPIs for hotel review processing",
		Category:    "business",
		RefreshRate: 30 * time.Second,
		Tags:        []string{"business", "kpi", "overview"},
		Filters: []DashboardFilter{
			{Name: "time_range", Type: "dropdown", Options: []string{"1h", "6h", "24h", "7d"}, Default: "24h"},
			{Name: "provider", Type: "dropdown", Options: []string{"all", "booking.com", "expedia", "hotels.com"}, Default: "all"},
		},
		Panels: []DashboardPanel{
			{
				ID:    "reviews-ingested-rate",
				Title: "Reviews Ingested (per minute)",
				Type:  "stat",
				Query: "rate(hotel_reviews_ingested_total[5m]) * 60",
				Unit:  "reviews/min",
				Position: PanelPosition{X: 0, Y: 0, Width: 6, Height: 4},
				Thresholds: []Threshold{
					{Value: 10, Color: "red", State: "critical"},
					{Value: 50, Color: "yellow", State: "warning"},
					{Value: 100, Color: "green", State: "ok"},
				},
			},
			{
				ID:    "reviews-processed-success-rate",
				Title: "Review Processing Success Rate",
				Type:  "stat",
				Query: "rate(hotel_reviews_stored_total[5m]) / rate(hotel_reviews_ingested_total[5m]) * 100",
				Unit:  "percent",
				Position: PanelPosition{X: 6, Y: 0, Width: 6, Height: 4},
				Thresholds: []Threshold{
					{Value: 95, Color: "green", State: "ok"},
					{Value: 90, Color: "yellow", State: "warning"},
					{Value: 85, Color: "red", State: "critical"},
				},
			},
			{
				ID:    "average-rating-distribution",
				Title: "Average Rating Distribution by Provider",
				Type:  "graph",
				Query: "avg(hotel_reviews_average_rating_by_hotel) by (provider)",
				Unit:  "rating",
				Position: PanelPosition{X: 0, Y: 4, Width: 12, Height: 6},
			},
			{
				ID:    "top-rated-hotels",
				Title: "Top Rated Hotels (Last 24h)",
				Type:  "table",
				Query: "topk(10, avg_over_time(hotel_reviews_average_rating_by_hotel[24h]))",
				Unit:  "rating",
				Position: PanelPosition{X: 0, Y: 10, Width: 6, Height: 8},
			},
			{
				ID:    "review-volume-by-provider",
				Title: "Review Volume by Provider",
				Type:  "graph",
				Query: "sum(rate(hotel_reviews_ingested_total[5m])) by (provider)",
				Unit:  "reviews/sec",
				Position: PanelPosition{X: 6, Y: 10, Width: 6, Height: 8},
			},
		},
	}
}

// createPerformanceMonitoringDashboard creates the performance monitoring dashboard
func (dm *DashboardManager) createPerformanceMonitoringDashboard() *DashboardConfig {
	return &DashboardConfig{
		ID:          "performance-monitoring",
		Title:       "Hotel Reviews - Performance Monitoring",
		Description: "System performance, latency, and throughput metrics",
		Category:    "performance",
		RefreshRate: 15 * time.Second,
		Tags:        []string{"performance", "latency", "throughput"},
		Panels: []DashboardPanel{
			{
				ID:    "api-response-time",
				Title: "API Response Time (95th percentile)",
				Type:  "graph",
				Query: "histogram_quantile(0.95, rate(hotel_reviews_http_request_duration_seconds_bucket[5m]))",
				Unit:  "seconds",
				Position: PanelPosition{X: 0, Y: 0, Width: 6, Height: 6},
				Thresholds: []Threshold{
					{Value: 0.1, Color: "green", State: "ok"},
					{Value: 0.5, Color: "yellow", State: "warning"},
					{Value: 1.0, Color: "red", State: "critical"},
				},
			},
			{
				ID:    "database-query-time",
				Title: "Database Query Time (95th percentile)",
				Type:  "graph",
				Query: "histogram_quantile(0.95, rate(hotel_reviews_database_query_duration_seconds_bucket[5m]))",
				Unit:  "seconds",
				Position: PanelPosition{X: 6, Y: 0, Width: 6, Height: 6},
			},
			{
				ID:    "file-processing-time",
				Title: "File Processing Time Distribution",
				Type:  "graph",
				Query: "histogram_quantile(0.5, rate(hotel_reviews_file_processing_time_bucket[5m]))",
				Unit:  "seconds",
				Position: PanelPosition{X: 0, Y: 6, Width: 12, Height: 6},
			},
			{
				ID:    "cache-hit-ratio",
				Title: "Cache Hit Ratio",
				Type:  "stat",
				Query: "rate(hotel_reviews_cache_hits_total[5m]) / (rate(hotel_reviews_cache_hits_total[5m]) + rate(hotel_reviews_cache_misses_total[5m])) * 100",
				Unit:  "percent",
				Position: PanelPosition{X: 0, Y: 12, Width: 4, Height: 4},
				Thresholds: []Threshold{
					{Value: 80, Color: "green", State: "ok"},
					{Value: 60, Color: "yellow", State: "warning"},
					{Value: 40, Color: "red", State: "critical"},
				},
			},
			{
				ID:    "processing-capacity",
				Title: "Processing Capacity Utilization",
				Type:  "gauge",
				Query: "hotel_reviews_processing_utilization",
				Unit:  "percent",
				Position: PanelPosition{X: 4, Y: 12, Width: 4, Height: 4},
			},
			{
				ID:    "queue-depth",
				Title: "Processing Queue Depth",
				Type:  "stat",
				Query: "hotel_reviews_queue_depth",
				Unit:  "jobs",
				Position: PanelPosition{X: 8, Y: 12, Width: 4, Height: 4},
			},
		},
	}
}

// createDataQualityDashboard creates the data quality monitoring dashboard
func (dm *DashboardManager) createDataQualityDashboard() *DashboardConfig {
	return &DashboardConfig{
		ID:          "data-quality",
		Title:       "Hotel Reviews - Data Quality",
		Description: "Data quality metrics, validation results, and anomaly detection",
		Category:    "quality",
		RefreshRate: 60 * time.Second,
		Tags:        []string{"quality", "validation", "anomaly"},
		Panels: []DashboardPanel{
			{
				ID:    "data-quality-score",
				Title: "Overall Data Quality Score",
				Type:  "gauge",
				Query: "avg(hotel_reviews_data_quality_score)",
				Unit:  "percent",
				Position: PanelPosition{X: 0, Y: 0, Width: 6, Height: 6},
				Thresholds: []Threshold{
					{Value: 95, Color: "green", State: "ok"},
					{Value: 85, Color: "yellow", State: "warning"},
					{Value: 75, Color: "red", State: "critical"},
				},
			},
			{
				ID:    "validation-failure-rate",
				Title: "Validation Failure Rate",
				Type:  "stat",
				Query: "rate(hotel_reviews_rejected_total[5m]) / rate(hotel_reviews_ingested_total[5m]) * 100",
				Unit:  "percent",
				Position: PanelPosition{X: 6, Y: 0, Width: 6, Height: 6},
			},
			{
				ID:    "missing-fields-breakdown",
				Title: "Missing Fields Breakdown",
				Type:  "table",
				Query: "sum(rate(hotel_reviews_missing_fields_total[5m])) by (field_name)",
				Unit:  "count/sec",
				Position: PanelPosition{X: 0, Y: 6, Width: 6, Height: 8},
			},
			{
				ID:    "provider-data-quality",
				Title: "Data Quality by Provider",
				Type:  "graph",
				Query: "avg(hotel_reviews_provider_data_quality) by (provider)",
				Unit:  "score",
				Position: PanelPosition{X: 6, Y: 6, Width: 6, Height: 8},
			},
			{
				ID:    "deduplication-hits",
				Title: "Duplicate Review Detection Rate",
				Type:  "stat",
				Query: "rate(hotel_reviews_deduplication_hits_total[5m])",
				Unit:  "duplicates/sec",
				Position: PanelPosition{X: 0, Y: 14, Width: 6, Height: 4},
			},
			{
				ID:    "data-completeness",
				Title: "Data Completeness Score",
				Type:  "stat",
				Query: "avg(hotel_reviews_data_completeness)",
				Unit:  "percent",
				Position: PanelPosition{X: 6, Y: 14, Width: 6, Height: 4},
			},
		},
	}
}

// createOperationalHealthDashboard creates the operational health dashboard
func (dm *DashboardManager) createOperationalHealthDashboard() *DashboardConfig {
	return &DashboardConfig{
		ID:          "operational-health",
		Title:       "Hotel Reviews - Operational Health",
		Description: "System health, error rates, and infrastructure monitoring",
		Category:    "operations",
		RefreshRate: 10 * time.Second,
		Tags:        []string{"health", "errors", "infrastructure"},
		Panels: []DashboardPanel{
			{
				ID:    "error-rate",
				Title: "Overall Error Rate",
				Type:  "stat",
				Query: "rate(hotel_reviews_processing_errors_total[5m])",
				Unit:  "errors/sec",
				Position: PanelPosition{X: 0, Y: 0, Width: 4, Height: 4},
				Thresholds: []Threshold{
					{Value: 0.1, Color: "green", State: "ok"},
					{Value: 1.0, Color: "yellow", State: "warning"},
					{Value: 5.0, Color: "red", State: "critical"},
				},
			},
			{
				ID:    "http-status-codes",
				Title: "HTTP Status Code Distribution",
				Type:  "graph",
				Query: "sum(rate(hotel_reviews_http_requests_total[5m])) by (status_code)",
				Unit:  "requests/sec",
				Position: PanelPosition{X: 4, Y: 0, Width: 8, Height: 6},
			},
			{
				ID:    "database-connections",
				Title: "Database Connection Pool",
				Type:  "graph",
				Query: "hotel_reviews_database_connections",
				Unit:  "connections",
				Position: PanelPosition{X: 0, Y: 6, Width: 6, Height: 6},
			},
			{
				ID:    "memory-usage",
				Title: "Memory Usage",
				Type:  "graph",
				Query: "hotel_reviews_memory_usage_bytes",
				Unit:  "bytes",
				Position: PanelPosition{X: 6, Y: 6, Width: 6, Height: 6},
			},
			{
				ID:    "circuit-breaker-state",
				Title: "Circuit Breaker States",
				Type:  "table",
				Query: "hotel_reviews_circuit_breaker_state",
				Unit:  "state",
				Position: PanelPosition{X: 0, Y: 12, Width: 6, Height: 6},
			},
			{
				ID:    "s3-operation-errors",
				Title: "S3 Operation Error Rate",
				Type:  "stat",
				Query: "rate(hotel_reviews_s3_errors_total[5m])",
				Unit:  "errors/sec",
				Position: PanelPosition{X: 6, Y: 12, Width: 6, Height: 6},
			},
		},
	}
}

// createCustomerInsightsDashboard creates the customer insights dashboard
func (dm *DashboardManager) createCustomerInsightsDashboard() *DashboardConfig {
	return &DashboardConfig{
		ID:          "customer-insights",
		Title:       "Hotel Reviews - Customer Insights",
		Description: "Customer behavior, satisfaction metrics, and trend analysis",
		Category:    "insights",
		RefreshRate: 5 * time.Minute,
		Tags:        []string{"customer", "satisfaction", "trends"},
		Panels: []DashboardPanel{
			{
				ID:    "customer-satisfaction-score",
				Title: "Overall Customer Satisfaction Score",
				Type:  "gauge",
				Query: "avg(hotel_reviews_customer_satisfaction_score)",
				Unit:  "score",
				Position: PanelPosition{X: 0, Y: 0, Width: 6, Height: 6},
				Thresholds: []Threshold{
					{Value: 4.0, Color: "green", State: "ok"},
					{Value: 3.5, Color: "yellow", State: "warning"},
					{Value: 3.0, Color: "red", State: "critical"},
				},
			},
			{
				ID:    "review-sentiment-distribution",
				Title: "Review Sentiment Distribution",
				Type:  "graph",
				Query: "sum(rate(hotel_reviews_sentiment_distribution_total[1h])) by (sentiment)",
				Unit:  "reviews/hour",
				Position: PanelPosition{X: 6, Y: 0, Width: 6, Height: 6},
			},
			{
				ID:    "reviews-per-user",
				Title: "Reviews per User Distribution",
				Type:  "graph",
				Query: "histogram_quantile(0.95, hotel_reviews_per_user_bucket)",
				Unit:  "reviews",
				Position: PanelPosition{X: 0, Y: 6, Width: 12, Height: 6},
			},
			{
				ID:    "review-velocity-trend",
				Title: "Review Velocity Trend (7-day moving average)",
				Type:  "graph",
				Query: "avg_over_time(hotel_reviews_velocity[7d])",
				Unit:  "reviews/day",
				Position: PanelPosition{X: 0, Y: 12, Width: 12, Height: 6},
			},
			{
				ID:    "user-activity-patterns",
				Title: "User Activity Patterns by Hour",
				Type:  "graph",
				Query: "sum(rate(hotel_reviews_user_activity_total[1h])) by (hour)",
				Unit:  "activity/hour",
				Position: PanelPosition{X: 0, Y: 18, Width: 12, Height: 6},
			},
		},
	}
}

// AddDashboard adds a dashboard to the manager
func (dm *DashboardManager) AddDashboard(config *DashboardConfig) {
	dm.dashboards[config.ID] = config
	dm.logger.Info("Dashboard added", "id", config.ID, "title", config.Title)
}

// GetDashboard retrieves a dashboard by ID
func (dm *DashboardManager) GetDashboard(id string) (*DashboardConfig, bool) {
	dashboard, exists := dm.dashboards[id]
	return dashboard, exists
}

// ListDashboards returns all available dashboards
func (dm *DashboardManager) ListDashboards() map[string]*DashboardConfig {
	return dm.dashboards
}

// GetDashboardsByCategory returns dashboards filtered by category
func (dm *DashboardManager) GetDashboardsByCategory(category string) []*DashboardConfig {
	var dashboards []*DashboardConfig
	for _, dashboard := range dm.dashboards {
		if dashboard.Category == category {
			dashboards = append(dashboards, dashboard)
		}
	}
	return dashboards
}

// UpdateDashboard updates an existing dashboard
func (dm *DashboardManager) UpdateDashboard(config *DashboardConfig) error {
	if _, exists := dm.dashboards[config.ID]; !exists {
		return fmt.Errorf("dashboard with ID %s not found", config.ID)
	}
	dm.dashboards[config.ID] = config
	dm.logger.Info("Dashboard updated", "id", config.ID)
	return nil
}

// DeleteDashboard removes a dashboard
func (dm *DashboardManager) DeleteDashboard(id string) error {
	if _, exists := dm.dashboards[id]; !exists {
		return fmt.Errorf("dashboard with ID %s not found", id)
	}
	delete(dm.dashboards, id)
	dm.logger.Info("Dashboard deleted", "id", id)
	return nil
}

// ExportDashboard exports a dashboard configuration as JSON
func (dm *DashboardManager) ExportDashboard(id string) ([]byte, error) {
	dashboard, exists := dm.GetDashboard(id)
	if !exists {
		return nil, fmt.Errorf("dashboard with ID %s not found", id)
	}
	return json.MarshalIndent(dashboard, "", "  ")
}

// ImportDashboard imports a dashboard from JSON
func (dm *DashboardManager) ImportDashboard(data []byte) error {
	var config DashboardConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse dashboard JSON: %w", err)
	}
	dm.AddDashboard(&config)
	return nil
}

// GetDashboardHTTPHandler returns an HTTP handler for dashboard management
func (dm *DashboardManager) GetDashboardHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			dm.handleGetDashboards(w, r)
		case http.MethodPost:
			dm.handleCreateDashboard(w, r)
		case http.MethodPut:
			dm.handleUpdateDashboard(w, r)
		case http.MethodDelete:
			dm.handleDeleteDashboard(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

// handleGetDashboards handles GET requests for dashboards
func (dm *DashboardManager) handleGetDashboards(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	id := r.URL.Query().Get("id")
	
	w.Header().Set("Content-Type", "application/json")
	
	if id != "" {
		// Get specific dashboard
		dashboard, exists := dm.GetDashboard(id)
		if !exists {
			http.Error(w, "Dashboard not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(dashboard)
		return
	}
	
	if category != "" {
		// Get dashboards by category
		dashboards := dm.GetDashboardsByCategory(category)
		json.NewEncoder(w).Encode(dashboards)
		return
	}
	
	// Get all dashboards
	dashboards := dm.ListDashboards()
	json.NewEncoder(w).Encode(dashboards)
}

// handleCreateDashboard handles POST requests to create dashboards
func (dm *DashboardManager) handleCreateDashboard(w http.ResponseWriter, r *http.Request) {
	var config DashboardConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	dm.AddDashboard(&config)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "id": config.ID})
}

// handleUpdateDashboard handles PUT requests to update dashboards
func (dm *DashboardManager) handleUpdateDashboard(w http.ResponseWriter, r *http.Request) {
	var config DashboardConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	if err := dm.UpdateDashboard(&config); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(map[string]string{"status": "updated", "id": config.ID})
}

// handleDeleteDashboard handles DELETE requests to remove dashboards
func (dm *DashboardManager) handleDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Dashboard ID required", http.StatusBadRequest)
		return
	}
	
	if err := dm.DeleteDashboard(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id})
}