package monitoring

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDashboardManager(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	
	dm := NewDashboardManager(logger)
	
	assert.NotNil(t, dm)
	assert.NotNil(t, dm.dashboards)
	assert.Equal(t, logger, dm.logger)
	
	// Check that default dashboards are created
	dashboards := dm.ListDashboards()
	assert.Len(t, dashboards, 5) // business-overview, performance-monitoring, data-quality, operational-health, customer-insights
	
	// Verify specific dashboards exist
	_, exists := dm.GetDashboard("business-overview")
	assert.True(t, exists)
	
	_, exists = dm.GetDashboard("performance-monitoring")
	assert.True(t, exists)
	
	_, exists = dm.GetDashboard("data-quality")
	assert.True(t, exists)
}

func TestDashboardManager_AddAndGetDashboard(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dm := NewDashboardManager(logger)
	
	// Create a test dashboard
	testDashboard := &DashboardConfig{
		ID:          "test-dashboard",
		Title:       "Test Dashboard",
		Description: "A test dashboard for unit testing",
		Category:    "test",
		Panels: []DashboardPanel{
			{
				ID:    "test-panel",
				Title: "Test Panel",
				Type:  "stat",
				Query: "test_metric",
				Position: PanelPosition{X: 0, Y: 0, Width: 6, Height: 4},
			},
		},
	}
	
	// Add the dashboard
	dm.AddDashboard(testDashboard)
	
	// Retrieve the dashboard
	retrieved, exists := dm.GetDashboard("test-dashboard")
	assert.True(t, exists)
	assert.Equal(t, testDashboard, retrieved)
}

func TestDashboardManager_GetDashboardsByCategory(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dm := NewDashboardManager(logger)
	
	// Get business category dashboards
	businessDashboards := dm.GetDashboardsByCategory("business")
	assert.Len(t, businessDashboards, 1)
	assert.Equal(t, "business-overview", businessDashboards[0].ID)
	
	// Get performance category dashboards
	performanceDashboards := dm.GetDashboardsByCategory("performance")
	assert.Len(t, performanceDashboards, 1)
	assert.Equal(t, "performance-monitoring", performanceDashboards[0].ID)
}

func TestDashboardManager_UpdateDashboard(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dm := NewDashboardManager(logger)
	
	// Get existing dashboard
	dashboard, exists := dm.GetDashboard("business-overview")
	require.True(t, exists)
	
	// Update the dashboard
	dashboard.Description = "Updated description"
	err := dm.UpdateDashboard(dashboard)
	assert.NoError(t, err)
	
	// Verify the update
	updated, exists := dm.GetDashboard("business-overview")
	assert.True(t, exists)
	assert.Equal(t, "Updated description", updated.Description)
	
	// Try to update non-existent dashboard
	nonExistent := &DashboardConfig{ID: "non-existent"}
	err = dm.UpdateDashboard(nonExistent)
	assert.Error(t, err)
}

func TestDashboardManager_DeleteDashboard(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dm := NewDashboardManager(logger)
	
	// Add a test dashboard
	testDashboard := &DashboardConfig{
		ID:    "delete-test",
		Title: "Delete Test Dashboard",
	}
	dm.AddDashboard(testDashboard)
	
	// Verify it exists
	_, exists := dm.GetDashboard("delete-test")
	assert.True(t, exists)
	
	// Delete it
	err := dm.DeleteDashboard("delete-test")
	assert.NoError(t, err)
	
	// Verify it's gone
	_, exists = dm.GetDashboard("delete-test")
	assert.False(t, exists)
	
	// Try to delete non-existent dashboard
	err = dm.DeleteDashboard("non-existent")
	assert.Error(t, err)
}

func TestDashboardManager_ExportImportDashboard(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dm := NewDashboardManager(logger)
	
	// Export an existing dashboard
	exportData, err := dm.ExportDashboard("business-overview")
	assert.NoError(t, err)
	assert.NotEmpty(t, exportData)
	
	// Verify it's valid JSON
	var config DashboardConfig
	err = json.Unmarshal(exportData, &config)
	assert.NoError(t, err)
	assert.Equal(t, "business-overview", config.ID)
	
	// Try to export non-existent dashboard
	_, err = dm.ExportDashboard("non-existent")
	assert.Error(t, err)
	
	// Test import
	testConfig := DashboardConfig{
		ID:    "imported-dashboard",
		Title: "Imported Dashboard",
	}
	testData, _ := json.Marshal(testConfig)
	
	err = dm.ImportDashboard(testData)
	assert.NoError(t, err)
	
	// Verify the import
	imported, exists := dm.GetDashboard("imported-dashboard")
	assert.True(t, exists)
	assert.Equal(t, "Imported Dashboard", imported.Title)
	
	// Test import with invalid JSON
	err = dm.ImportDashboard([]byte("invalid json"))
	assert.Error(t, err)
}

func TestDashboardManager_HTTPHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dm := NewDashboardManager(logger)
	handler := dm.GetDashboardHTTPHandler()
	
	t.Run("GET all dashboards", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dashboards", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		
		var dashboards map[string]*DashboardConfig
		err := json.NewDecoder(rec.Body).Decode(&dashboards)
		assert.NoError(t, err)
		assert.Len(t, dashboards, 5) // Default dashboards
	})
	
	t.Run("GET dashboard by ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dashboards?id=business-overview", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var dashboard DashboardConfig
		err := json.NewDecoder(rec.Body).Decode(&dashboard)
		assert.NoError(t, err)
		assert.Equal(t, "business-overview", dashboard.ID)
	})
	
	t.Run("GET dashboards by category", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dashboards?category=business", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var dashboards []*DashboardConfig
		err := json.NewDecoder(rec.Body).Decode(&dashboards)
		assert.NoError(t, err)
		assert.Len(t, dashboards, 1)
		assert.Equal(t, "business-overview", dashboards[0].ID)
	})
	
	t.Run("POST create dashboard", func(t *testing.T) {
		newDashboard := DashboardConfig{
			ID:    "new-dashboard",
			Title: "New Dashboard",
		}
		
		body, _ := json.Marshal(newDashboard)
		req := httptest.NewRequest("POST", "/dashboards", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusCreated, rec.Code)
		
		// Verify it was created
		_, exists := dm.GetDashboard("new-dashboard")
		assert.True(t, exists)
	})
	
	t.Run("GET non-existent dashboard", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dashboards?id=non-existent", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
	
	t.Run("unsupported method", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/dashboards", nil)
		rec := httptest.NewRecorder()
		
		handler.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}

func TestCreateBusinessOverviewDashboard(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dm := NewDashboardManager(logger)
	
	dashboard := dm.createBusinessOverviewDashboard()
	
	assert.Equal(t, "business-overview", dashboard.ID)
	assert.Equal(t, "Hotel Reviews - Business Overview", dashboard.Title)
	assert.Equal(t, "business", dashboard.Category)
	assert.NotEmpty(t, dashboard.Panels)
	assert.NotEmpty(t, dashboard.Filters)
	
	// Check specific panels exist
	panelIDs := make([]string, len(dashboard.Panels))
	for i, panel := range dashboard.Panels {
		panelIDs[i] = panel.ID
	}
	
	assert.Contains(t, panelIDs, "reviews-ingested-rate")
	assert.Contains(t, panelIDs, "reviews-processed-success-rate")
	assert.Contains(t, panelIDs, "average-rating-distribution")
}

func TestCreatePerformanceMonitoringDashboard(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dm := NewDashboardManager(logger)
	
	dashboard := dm.createPerformanceMonitoringDashboard()
	
	assert.Equal(t, "performance-monitoring", dashboard.ID)
	assert.Equal(t, "performance", dashboard.Category)
	assert.NotEmpty(t, dashboard.Panels)
	
	// Check for performance-specific panels
	panelIDs := make([]string, len(dashboard.Panels))
	for i, panel := range dashboard.Panels {
		panelIDs[i] = panel.ID
	}
	
	assert.Contains(t, panelIDs, "api-response-time")
	assert.Contains(t, panelIDs, "database-query-time")
	assert.Contains(t, panelIDs, "cache-hit-ratio")
}

func TestDashboardPanelValidation(t *testing.T) {
	panel := DashboardPanel{
		ID:    "test-panel",
		Title: "Test Panel",
		Type:  "stat",
		Query: "test_metric",
		Unit:  "count",
		Position: PanelPosition{
			X:      0,
			Y:      0,
			Width:  6,
			Height: 4,
		},
		Thresholds: []Threshold{
			{Value: 10, Color: "green", State: "ok"},
			{Value: 20, Color: "yellow", State: "warning"},
			{Value: 30, Color: "red", State: "critical"},
		},
	}
	
	assert.Equal(t, "test-panel", panel.ID)
	assert.Equal(t, "stat", panel.Type)
	assert.Equal(t, "test_metric", panel.Query)
	assert.Len(t, panel.Thresholds, 3)
	assert.Equal(t, 6, panel.Position.Width)
}