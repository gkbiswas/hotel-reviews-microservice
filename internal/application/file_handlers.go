package application

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
	"github.com/google/uuid"
)

// FileHandlers handles file upload and processing operations
type FileHandlers struct {
	reviewService    domain.ReviewService
	s3Client        domain.S3Client
	jsonProcessor   domain.JSONProcessor
	processingEngine *ProcessingEngine
	logger          *logger.Logger
	bucketName      string
	maxFileSize     int64
}

// NewFileHandlers creates a new instance of FileHandlers
func NewFileHandlers(
	reviewService domain.ReviewService,
	s3Client domain.S3Client,
	jsonProcessor domain.JSONProcessor,
	processingEngine *ProcessingEngine,
	logger *logger.Logger,
	bucketName string,
) *FileHandlers {
	return &FileHandlers{
		reviewService:    reviewService,
		s3Client:        s3Client,
		jsonProcessor:   jsonProcessor,
		processingEngine: processingEngine,
		logger:          logger,
		bucketName:      bucketName,
		maxFileSize:     100 * 1024 * 1024, // 100MB default
	}
}

// UploadAndProcessFile handles POST /api/v1/files/upload
func (f *FileHandlers) UploadAndProcessFile(c *gin.Context) {
	// Parse provider ID from form data or query
	providerIDStr := c.PostForm("provider_id")
	if providerIDStr == "" {
		providerIDStr = c.Query("provider_id")
	}
	
	if providerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_PROVIDER_ID",
			"message": "Provider ID is required",
		})
		return
	}

	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_PROVIDER_ID",
			"message": "Invalid provider ID format",
		})
		return
	}

	// Get the file from form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "FILE_MISSING",
			"message": "No file uploaded",
		})
		return
	}
	defer file.Close()

	// Validate file
	if err := f.validateFile(header); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "FILE_VALIDATION_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Generate unique file key
	fileKey := f.generateFileKey(providerID, header.Filename)

	// Upload to S3
	if err := f.uploadToS3(c.Request.Context(), file, fileKey, header); err != nil {
		f.logger.ErrorContext(c.Request.Context(), "Failed to upload file to S3", 
			"error", err, 
			"provider_id", providerID,
			"filename", header.Filename)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "UPLOAD_FAILED",
			"message": "Failed to upload file",
		})
		return
	}

	// Generate file URL
	fileURL := fmt.Sprintf("s3://%s/%s", f.bucketName, fileKey)

	// Start processing
	processingStatus, err := f.reviewService.ProcessReviewFile(c.Request.Context(), fileURL, providerID)
	if err != nil {
		f.logger.ErrorContext(c.Request.Context(), "Failed to start file processing", 
			"error", err, 
			"provider_id", providerID,
			"file_url", fileURL)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "PROCESSING_FAILED",
			"message": "Failed to start file processing",
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "File uploaded and processing started",
		"data": gin.H{
			"processing_id": processingStatus.ID,
			"status":        processingStatus.Status,
			"file_url":      fileURL,
			"file_name":     header.Filename,
			"file_size":     header.Size,
		},
	})
}

// GetProcessingStatus handles GET /api/v1/files/processing/:id
func (f *FileHandlers) GetProcessingStatus(c *gin.Context) {
	processingIDStr := c.Param("id")
	processingID, err := uuid.Parse(processingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid processing ID format",
		})
		return
	}

	status, err := f.reviewService.GetProcessingStatus(c.Request.Context(), processingID)
	if err != nil {
		if err == domain.ErrProcessingNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "PROCESSING_NOT_FOUND",
				"message": "Processing job not found",
			})
			return
		}

		f.logger.ErrorContext(c.Request.Context(), "Failed to get processing status", 
			"error", err, 
			"processing_id", processingID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve processing status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": status,
	})
}

// GetProcessingHistory handles GET /api/v1/files/processing/history
func (f *FileHandlers) GetProcessingHistory(c *gin.Context) {
	// Parse provider ID
	providerIDStr := c.Query("provider_id")
	if providerIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_PROVIDER_ID",
			"message": "Provider ID is required",
		})
		return
	}

	providerID, err := uuid.Parse(providerIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_PROVIDER_ID",
			"message": "Invalid provider ID format",
		})
		return
	}

	// Parse pagination
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	history, err := f.reviewService.GetProcessingHistory(c.Request.Context(), providerID, limit, offset)
	if err != nil {
		f.logger.ErrorContext(c.Request.Context(), "Failed to get processing history", 
			"error", err, 
			"provider_id", providerID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to retrieve processing history",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": history,
		"meta": gin.H{
			"limit":  limit,
			"offset": offset,
			"count":  len(history),
		},
	})
}

// CancelProcessing handles POST /api/v1/files/processing/:id/cancel
func (f *FileHandlers) CancelProcessing(c *gin.Context) {
	processingIDStr := c.Param("id")
	processingID, err := uuid.Parse(processingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_ID",
			"message": "Invalid processing ID format",
		})
		return
	}

	if err := f.reviewService.CancelProcessing(c.Request.Context(), processingID); err != nil {
		if err == domain.ErrProcessingNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "PROCESSING_NOT_FOUND",
				"message": "Processing job not found",
			})
			return
		}

		f.logger.ErrorContext(c.Request.Context(), "Failed to cancel processing", 
			"error", err, 
			"processing_id", processingID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "INTERNAL_ERROR",
			"message": "Failed to cancel processing",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Processing cancelled successfully",
	})
}

// ValidateFile handles POST /api/v1/files/validate
func (f *FileHandlers) ValidateFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "FILE_MISSING",
			"message": "No file uploaded",
		})
		return
	}
	defer file.Close()

	// Basic file validation
	if err := f.validateFile(header); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "FILE_VALIDATION_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Validate file content
	recordCount := 0
	validationErrors := []string{}

	// Create a temporary reader to count and validate records
	if err := f.jsonProcessor.ValidateFile(c.Request.Context(), file); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}

	// Reset file reader
	file.Seek(0, 0)

	// Count records
	count, err := f.jsonProcessor.CountRecords(c.Request.Context(), file)
	if err == nil {
		recordCount = count
	}

	isValid := len(validationErrors) == 0

	c.JSON(http.StatusOK, gin.H{
		"valid":        isValid,
		"record_count": recordCount,
		"errors":       validationErrors,
		"file_info": gin.H{
			"name": header.Filename,
			"size": header.Size,
			"type": header.Header.Get("Content-Type"),
		},
	})
}

// GetProcessingMetrics handles GET /api/v1/files/metrics
func (f *FileHandlers) GetProcessingMetrics(c *gin.Context) {
	metrics := f.processingEngine.GetMetrics()
	
	c.JSON(http.StatusOK, gin.H{
		"data": metrics,
	})
}

// Helper methods

func (f *FileHandlers) validateFile(header *multipart.FileHeader) error {
	// Check file size
	if header.Size > f.maxFileSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d MB", f.maxFileSize/(1024*1024))
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jsonl" && ext != ".json" {
		return fmt.Errorf("unsupported file format. Only .jsonl and .json files are allowed")
	}

	// Check content type
	contentType := header.Header.Get("Content-Type")
	if contentType != "" && !strings.Contains(contentType, "json") && contentType != "application/octet-stream" {
		return fmt.Errorf("invalid content type: %s", contentType)
	}

	return nil
}

func (f *FileHandlers) generateFileKey(providerID uuid.UUID, filename string) string {
	timestamp := time.Now().Format("20060102-150405")
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)
	
	// Clean filename
	baseName = strings.ReplaceAll(baseName, " ", "_")
	baseName = strings.ReplaceAll(baseName, "/", "_")
	
	return fmt.Sprintf("reviews/%s/%s_%s_%s%s", 
		providerID.String(),
		timestamp,
		baseName,
		uuid.New().String()[:8],
		ext)
}

func (f *FileHandlers) uploadToS3(ctx context.Context, file multipart.File, key string, header *multipart.FileHeader) error {
	contentType := "application/json"
	if ct := header.Header.Get("Content-Type"); ct != "" {
		contentType = ct
	}

	return f.s3Client.UploadFile(ctx, f.bucketName, key, file, contentType)
}