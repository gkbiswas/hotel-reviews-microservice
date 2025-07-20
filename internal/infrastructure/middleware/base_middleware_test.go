package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRequestID(t *testing.T) {
	t.Run("generates new request ID when not provided", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		
		handlerCalled := false
		var requestID string
		router.GET("/test", func(c *gin.Context) {
			handlerCalled = true
			requestID = c.GetString("request_id")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.True(t, handlerCalled)
		assert.NotEmpty(t, requestID)
		assert.Equal(t, requestID, rec.Header().Get("X-Request-ID"))
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("uses existing request ID from header", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		
		existingID := "test-request-123"
		var capturedID string
		router.GET("/test", func(c *gin.Context) {
			capturedID = c.GetString("request_id")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", existingID)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, existingID, capturedID)
		assert.Equal(t, existingID, rec.Header().Get("X-Request-ID"))
	})

	t.Run("handles empty request ID header", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		
		var requestID string
		router.GET("/test", func(c *gin.Context) {
			requestID = c.GetString("request_id")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.NotEmpty(t, requestID)
		assert.Equal(t, requestID, rec.Header().Get("X-Request-ID"))
	})
}

func TestLogger(t *testing.T) {
	t.Run("logs successful request", func(t *testing.T) {
		router := gin.New()
		router.Use(Logger("test-logger"))
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("logs request with query parameters", func(t *testing.T) {
		router := gin.New()
		router.Use(Logger("test-logger"))
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test?param1=value1&param2=value2", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("logs request with errors", func(t *testing.T) {
		router := gin.New()
		router.Use(Logger("test-logger"))
		
		router.GET("/test", func(c *gin.Context) {
			c.Error(gin.Error{Err: assert.AnError, Type: gin.ErrorTypePrivate})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "test error"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("logs different HTTP methods", func(t *testing.T) {
		router := gin.New()
		router.Use(Logger("test-logger"))
		
		methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
		
		for _, method := range methods {
			router.Handle(method, "/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"method": method})
			})
		}

		for _, method := range methods {
			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
		}
	})
}

func TestErrorHandler(t *testing.T) {
	t.Run("handles no errors", func(t *testing.T) {
		router := gin.New()
		router.Use(ErrorHandler("test-logger"))
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "ok")
	})

	t.Run("handles single error", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID()) // Add request ID for error response
		router.Use(ErrorHandler("test-logger"))
		
		router.GET("/test", func(c *gin.Context) {
			c.Error(gin.Error{Err: assert.AnError, Type: gin.ErrorTypePrivate})
			c.Status(http.StatusBadRequest)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "error")
		assert.Contains(t, rec.Body.String(), "request_id")
	})

	t.Run("handles multiple errors and uses last one", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.Use(ErrorHandler("test-logger"))
		
		router.GET("/test", func(c *gin.Context) {
			c.Error(gin.Error{Err: assert.AnError, Type: gin.ErrorTypePrivate})
			c.Error(gin.Error{Err: errors.New("test error"), Type: gin.ErrorTypePrivate})
			c.Status(http.StatusUnprocessableEntity)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
		assert.Contains(t, rec.Body.String(), "error")
	})

	t.Run("sets status to 500 when no status code set", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.Use(ErrorHandler("test-logger"))
		
		router.GET("/test", func(c *gin.Context) {
			c.Error(gin.Error{Err: assert.AnError, Type: gin.ErrorTypePrivate})
			// Don't set status code
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "error")
	})
}

func TestSecurity(t *testing.T) {
	t.Run("adds all security headers", func(t *testing.T) {
		router := gin.New()
		router.Use(Security())
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		headers := rec.Header()
		assert.Equal(t, "nosniff", headers.Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", headers.Get("X-Frame-Options"))
		assert.Equal(t, "1; mode=block", headers.Get("X-XSS-Protection"))
		assert.Equal(t, "max-age=31536000; includeSubDomains", headers.Get("Strict-Transport-Security"))
		assert.Equal(t, "default-src 'self'", headers.Get("Content-Security-Policy"))
		assert.Equal(t, "none", headers.Get("X-Permitted-Cross-Domain-Policies"))
		assert.Equal(t, "no-referrer", headers.Get("Referrer-Policy"))
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("security headers don't interfere with response", func(t *testing.T) {
		router := gin.New()
		router.Use(Security())
		
		router.GET("/test", func(c *gin.Context) {
			c.Header("Custom-Header", "custom-value")
			c.JSON(http.StatusCreated, gin.H{"data": "response"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, "custom-value", rec.Header().Get("Custom-Header"))
		assert.Contains(t, rec.Body.String(), "response")
		// Security headers should still be present
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	})
}

func TestCORS(t *testing.T) {
	t.Run("allows request from allowed origin", func(t *testing.T) {
		allowedOrigins := []string{"https://example.com", "https://test.com"}
		router := gin.New()
		router.Use(CORS(allowedOrigins))
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", rec.Header().Get("Access-Control-Allow-Credentials"))
		assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Headers"))
		assert.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Methods"))
	})

	t.Run("allows wildcard origin", func(t *testing.T) {
		allowedOrigins := []string{"*"}
		router := gin.New()
		router.Use(CORS(allowedOrigins))
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://any-domain.com")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "https://any-domain.com", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("rejects request from disallowed origin", func(t *testing.T) {
		allowedOrigins := []string{"https://example.com"}
		router := gin.New()
		router.Use(CORS(allowedOrigins))
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://malicious.com")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code) // Request still processed
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("handles OPTIONS preflight request", func(t *testing.T) {
		allowedOrigins := []string{"https://example.com"}
		router := gin.New()
		router.Use(CORS(allowedOrigins))
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("OPTIONS", "/test", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("handles request without origin header", func(t *testing.T) {
		allowedOrigins := []string{"https://example.com"}
		router := gin.New()
		router.Use(CORS(allowedOrigins))
		
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestMetrics(t *testing.T) {
	t.Run("records metrics for successful request", func(t *testing.T) {
		router := gin.New()
		router.Use(Metrics("test-monitoring"))
		
		router.GET("/api/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/api/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		// Metrics should be recorded (we can't easily test Prometheus metrics here)
	})

	t.Run("records metrics for error request", func(t *testing.T) {
		router := gin.New()
		router.Use(Metrics("test-monitoring"))
		
		router.GET("/api/error", func(c *gin.Context) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "test error"})
		})

		req := httptest.NewRequest("GET", "/api/error", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("handles unknown route path", func(t *testing.T) {
		router := gin.New()
		router.Use(Metrics("test-monitoring"))
		
		// No route defined, should use "unknown" path
		req := httptest.NewRequest("GET", "/undefined", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("measures request duration", func(t *testing.T) {
		router := gin.New()
		router.Use(Metrics("test-monitoring"))
		
		router.GET("/slow", func(c *gin.Context) {
			time.Sleep(10 * time.Millisecond) // Simulate some work
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		start := time.Now()
		req := httptest.NewRequest("GET", "/slow", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, duration >= 10*time.Millisecond)
	})
}

func TestTracing(t *testing.T) {
	t.Run("adds tracing span to request", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID()) // Add request ID for tracing
		router.Use(Tracing("test-service"))
		
		router.GET("/api/test", func(c *gin.Context) {
			// Check that context is properly set
			assert.NotNil(t, c.Request.Context())
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/api/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("handles error status codes in tracing", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.Use(Tracing("test-service"))
		
		router.GET("/api/error", func(c *gin.Context) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request"})
		})

		req := httptest.NewRequest("GET", "/api/error", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("handles server error status codes in tracing", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.Use(Tracing("test-service"))
		
		router.GET("/api/server-error", func(c *gin.Context) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		})

		req := httptest.NewRequest("GET", "/api/server-error", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("preserves context throughout request", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.Use(Tracing("test-service"))
		
		var capturedContext interface{}
		router.GET("/api/context", func(c *gin.Context) {
			capturedContext = c.Request.Context()
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/api/context", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NotNil(t, capturedContext)
	})
}

func TestMiddlewareChaining(t *testing.T) {
	t.Run("middleware chain executes in correct order", func(t *testing.T) {
		var executionOrder []string
		
		router := gin.New()
		
		// Add middleware in specific order
		router.Use(func(c *gin.Context) {
			executionOrder = append(executionOrder, "first")
			c.Next()
			executionOrder = append(executionOrder, "first-after")
		})
		
		router.Use(RequestID())
		
		router.Use(func(c *gin.Context) {
			executionOrder = append(executionOrder, "second")
			c.Next()
			executionOrder = append(executionOrder, "second-after")
		})
		
		router.Use(Security())
		
		router.GET("/test", func(c *gin.Context) {
			executionOrder = append(executionOrder, "handler")
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, []string{
			"first",
			"second", 
			"handler",
			"second-after",
			"first-after",
		}, executionOrder)
		
		// Verify security headers and request ID are set
		assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	})

	t.Run("complete middleware stack works together", func(t *testing.T) {
		router := gin.New()
		
		// Add all middleware
		router.Use(RequestID())
		router.Use(Logger("test"))
		router.Use(Security())
		router.Use(CORS([]string{"*"}))
		router.Use(Metrics("test"))
		router.Use(Tracing("test-service"))
		router.Use(ErrorHandler("test"))
		
		router.GET("/api/complete", func(c *gin.Context) {
			requestID := c.GetString("request_id")
			assert.NotEmpty(t, requestID)
			c.JSON(http.StatusOK, gin.H{
				"status": "ok",
				"request_id": requestID,
			})
		})

		req := httptest.NewRequest("GET", "/api/complete", nil)
		req.Header.Set("Origin", "https://example.com")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		
		// Verify all middleware effects
		assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
		assert.Equal(t, "https://example.com", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		assert.Contains(t, rec.Body.String(), "request_id")
	})
}

func TestConcurrentMiddlewareRequests(t *testing.T) {
	t.Run("middleware handles concurrent requests safely", func(t *testing.T) {
		router := gin.New()
		router.Use(RequestID())
		router.Use(Security())
		router.Use(Metrics("test"))
		
		requestCount := 0
		router.GET("/concurrent", func(c *gin.Context) {
			requestCount++
			requestID := c.GetString("request_id")
			assert.NotEmpty(t, requestID)
			c.JSON(http.StatusOK, gin.H{"request": requestCount})
		})

		numRequests := 10
		done := make(chan bool, numRequests)
		
		for i := 0; i < numRequests; i++ {
			go func(requestNum int) {
				defer func() { done <- true }()
				
				req := httptest.NewRequest("GET", "/concurrent", nil)
				rec := httptest.NewRecorder()
				
				router.ServeHTTP(rec, req)
				
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
				assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
			}(i)
		}

		for i := 0; i < numRequests; i++ {
			<-done
		}
	})
}