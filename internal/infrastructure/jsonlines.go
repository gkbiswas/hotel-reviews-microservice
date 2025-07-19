package infrastructure

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gkbiswas/hotel-reviews-microservice/internal/domain"
	"github.com/gkbiswas/hotel-reviews-microservice/pkg/logger"
)

// JSONLinesProcessor implements the domain.JSONProcessor interface
type JSONLinesProcessor struct {
	repository domain.ReviewRepository
	logger     *logger.Logger
	batchSize  int
	workerPool int
	validator  *ReviewValidator
}

// ReviewValidator handles validation of review data
type ReviewValidator struct {
	requiredFields []string
	maxCommentLen  int
	maxTitleLen    int
}

// ProcessingStats tracks processing statistics
type ProcessingStats struct {
	TotalLines     int64
	ProcessedLines int64
	ErrorLines     int64
	SkippedLines   int64
	StartTime      time.Time
	EndTime        time.Time
}

// LineError represents an error that occurred while processing a line
type LineError struct {
	LineNumber int64
	Content    string
	Error      error
}

// BatchJob represents a batch of lines to process
type BatchJob struct {
	Lines      []string
	StartLine  int64
	ProviderID uuid.UUID
}

// ProcessingResult represents the result of processing a batch
type ProcessingResult struct {
	Reviews      []domain.Review
	Hotels       []domain.Hotel
	ReviewerInfo []domain.ReviewerInfo
	Errors       []LineError
}

// ReviewJSON represents the JSON structure for reviews
type ReviewJSON struct {
	ID           string                 `json:"id"`
	HotelID      string                 `json:"hotel_id"`
	HotelName    string                 `json:"hotel_name"`
	Rating       interface{}            `json:"rating"`
	Title        string                 `json:"title"`
	Comment      string                 `json:"comment"`
	ReviewDate   string                 `json:"review_date"`
	StayDate     string                 `json:"stay_date,omitempty"`
	TripType     string                 `json:"trip_type,omitempty"`
	RoomType     string                 `json:"room_type,omitempty"`
	Language     string                 `json:"language,omitempty"`
	IsVerified   interface{}            `json:"is_verified,omitempty"`
	HelpfulVotes interface{}            `json:"helpful_votes,omitempty"`
	TotalVotes   interface{}            `json:"total_votes,omitempty"`
	
	// Reviewer information
	ReviewerName    string `json:"reviewer_name,omitempty"`
	ReviewerEmail   string `json:"reviewer_email,omitempty"`
	ReviewerCountry string `json:"reviewer_country,omitempty"`
	
	// Hotel information
	HotelAddress    string   `json:"hotel_address,omitempty"`
	HotelCity       string   `json:"hotel_city,omitempty"`
	HotelCountry    string   `json:"hotel_country,omitempty"`
	HotelPostalCode string   `json:"hotel_postal_code,omitempty"`
	HotelPhone      string   `json:"hotel_phone,omitempty"`
	HotelEmail      string   `json:"hotel_email,omitempty"`
	HotelStarRating interface{} `json:"hotel_star_rating,omitempty"`
	HotelAmenities  []string `json:"hotel_amenities,omitempty"`
	HotelLatitude   interface{} `json:"hotel_latitude,omitempty"`
	HotelLongitude  interface{} `json:"hotel_longitude,omitempty"`
	
	// Detailed ratings
	ServiceRating     interface{} `json:"service_rating,omitempty"`
	CleanlinessRating interface{} `json:"cleanliness_rating,omitempty"`
	LocationRating    interface{} `json:"location_rating,omitempty"`
	ValueRating       interface{} `json:"value_rating,omitempty"`
	ComfortRating     interface{} `json:"comfort_rating,omitempty"`
	FacilitiesRating  interface{} `json:"facilities_rating,omitempty"`
	
	// Additional metadata
	Source   string                 `json:"source,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewJSONLinesProcessor creates a new JSON Lines processor
func NewJSONLinesProcessor(repository domain.ReviewRepository, logger *logger.Logger) domain.JSONProcessor {
	validator := &ReviewValidator{
		requiredFields: []string{"id", "hotel_id", "hotel_name", "rating", "comment", "review_date"},
		maxCommentLen:  10000,
		maxTitleLen:    500,
	}
	
	return &JSONLinesProcessor{
		repository: repository,
		logger:     logger,
		batchSize:  1000,
		workerPool: 4,
		validator:  validator,
	}
}

// ProcessFile processes a JSON Lines file
func (j *JSONLinesProcessor) ProcessFile(ctx context.Context, reader io.Reader, providerID uuid.UUID, processingID uuid.UUID) error {
	start := time.Now()
	stats := &ProcessingStats{
		StartTime: start,
	}
	
	j.logger.InfoContext(ctx, "Starting JSON Lines file processing",
		"provider_id", providerID,
		"processing_id", processingID,
	)
	
	// Create buffered reader for efficient line-by-line reading
	scanner := bufio.NewScanner(reader)
	
	// Increase buffer size for large lines
	const maxCapacity = 1024 * 1024 // 1MB per line
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)
	
	// Process lines in batches using worker pool
	batchChan := make(chan BatchJob, j.workerPool)
	resultChan := make(chan ProcessingResult, j.workerPool)
	
	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < j.workerPool; i++ {
		wg.Add(1)
		go j.worker(ctx, batchChan, resultChan, &wg)
	}
	
	// Start result collector
	go j.resultCollector(ctx, resultChan, stats, processingID)
	
	// Read and batch lines
	var batch []string
	var lineNumber int64
	var batchStartLine int64
	
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines
		if line == "" {
			stats.SkippedLines++
			continue
		}
		
		// Add line to batch
		if len(batch) == 0 {
			batchStartLine = lineNumber
		}
		batch = append(batch, line)
		
		// Send batch when full
		if len(batch) >= j.batchSize {
			select {
			case batchChan <- BatchJob{
				Lines:      batch,
				StartLine:  batchStartLine,
				ProviderID: providerID,
			}:
				batch = make([]string, 0, j.batchSize)
			case <-ctx.Done():
				close(batchChan)
				return ctx.Err()
			}
		}
	}
	
	// Send remaining batch
	if len(batch) > 0 {
		select {
		case batchChan <- BatchJob{
			Lines:      batch,
			StartLine:  batchStartLine,
			ProviderID: providerID,
		}:
		case <-ctx.Done():
			close(batchChan)
			return ctx.Err()
		}
	}
	
	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		close(batchChan)
		return fmt.Errorf("error reading file: %w", err)
	}
	
	stats.TotalLines = lineNumber
	
	// Close batch channel and wait for workers
	close(batchChan)
	wg.Wait()
	close(resultChan)
	
	// Wait for result collector to finish
	time.Sleep(100 * time.Millisecond)
	
	stats.EndTime = time.Now()
	duration := stats.EndTime.Sub(stats.StartTime)
	
	j.logger.InfoContext(ctx, "JSON Lines file processing completed",
		"provider_id", providerID,
		"processing_id", processingID,
		"total_lines", stats.TotalLines,
		"processed_lines", stats.ProcessedLines,
		"error_lines", stats.ErrorLines,
		"skipped_lines", stats.SkippedLines,
		"duration_ms", duration.Milliseconds(),
		"lines_per_second", float64(stats.TotalLines)/duration.Seconds(),
	)
	
	return nil
}

// worker processes batches of lines
func (j *JSONLinesProcessor) worker(ctx context.Context, batchChan <-chan BatchJob, resultChan chan<- ProcessingResult, wg *sync.WaitGroup) {
	defer wg.Done()
	
	for batch := range batchChan {
		result := j.processBatch(ctx, batch)
		
		select {
		case resultChan <- result:
		case <-ctx.Done():
			return
		}
	}
}

// processBatch processes a batch of lines
func (j *JSONLinesProcessor) processBatch(ctx context.Context, batch BatchJob) ProcessingResult {
	result := ProcessingResult{
		Reviews:      make([]domain.Review, 0, len(batch.Lines)),
		Hotels:       make([]domain.Hotel, 0),
		ReviewerInfo: make([]domain.ReviewerInfo, 0),
		Errors:       make([]LineError, 0),
	}
	
	hotelMap := make(map[string]*domain.Hotel)
	reviewerMap := make(map[string]*domain.ReviewerInfo)
	
	for i, line := range batch.Lines {
		lineNumber := batch.StartLine + int64(i)
		
		review, hotel, reviewer, err := j.parseLine(ctx, line, batch.ProviderID)
		if err != nil {
			result.Errors = append(result.Errors, LineError{
				LineNumber: lineNumber,
				Content:    line,
				Error:      err,
			})
			continue
		}
		
		result.Reviews = append(result.Reviews, *review)
		
		// Collect unique hotels
		if hotel != nil {
			if _, exists := hotelMap[hotel.Name]; !exists {
				hotelMap[hotel.Name] = hotel
			}
		}
		
		// Collect unique reviewers
		if reviewer != nil && reviewer.Email != "" {
			if _, exists := reviewerMap[reviewer.Email]; !exists {
				reviewerMap[reviewer.Email] = reviewer
			}
		}
	}
	
	// Convert maps to slices
	for _, hotel := range hotelMap {
		result.Hotels = append(result.Hotels, *hotel)
	}
	for _, reviewer := range reviewerMap {
		result.ReviewerInfo = append(result.ReviewerInfo, *reviewer)
	}
	
	return result
}

// resultCollector collects processing results and saves to database
func (j *JSONLinesProcessor) resultCollector(ctx context.Context, resultChan <-chan ProcessingResult, stats *ProcessingStats, processingID uuid.UUID) {
	for result := range resultChan {
		// Save hotels first (upsert to handle duplicates)
		if len(result.Hotels) > 0 {
			for _, hotel := range result.Hotels {
				if err := j.repository.CreateHotel(ctx, &hotel); err != nil {
					j.logger.WarnContext(ctx, "Failed to upsert hotel",
						"hotel_name", hotel.Name,
						"error", err,
					)
				}
			}
		}
		
		// Save reviewer info (upsert to handle duplicates)
		if len(result.ReviewerInfo) > 0 {
			for _, reviewer := range result.ReviewerInfo {
				if err := j.repository.CreateReviewerInfo(ctx, &reviewer); err != nil {
					j.logger.WarnContext(ctx, "Failed to upsert reviewer info",
						"reviewer_email", reviewer.Email,
						"error", err,
					)
				}
			}
		}
		
		// Save reviews in batch
		if len(result.Reviews) > 0 {
			if err := j.repository.CreateBatch(ctx, result.Reviews); err != nil {
				j.logger.ErrorContext(ctx, "Failed to save review batch",
					"batch_size", len(result.Reviews),
					"error", err,
				)
				stats.ErrorLines += int64(len(result.Reviews))
			} else {
				stats.ProcessedLines += int64(len(result.Reviews))
			}
		}
		
		// Log errors
		if len(result.Errors) > 0 {
			for _, lineError := range result.Errors {
				j.logger.WarnContext(ctx, "Line processing error",
					"line_number", lineError.LineNumber,
					"error", lineError.Error,
				)
			}
			stats.ErrorLines += int64(len(result.Errors))
		}
		
		// Update processing status periodically
		if err := j.repository.UpdateProcessingStatus(ctx, processingID, "processing", int(stats.ProcessedLines), ""); err != nil {
			j.logger.WarnContext(ctx, "Failed to update processing status", "error", err)
		}
	}
}

// parseLine parses a single JSON line
func (j *JSONLinesProcessor) parseLine(ctx context.Context, line string, providerID uuid.UUID) (*domain.Review, *domain.Hotel, *domain.ReviewerInfo, error) {
	var reviewJSON ReviewJSON
	if err := json.Unmarshal([]byte(line), &reviewJSON); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	
	// Validate required fields
	if err := j.validator.validateReviewJSON(&reviewJSON); err != nil {
		return nil, nil, nil, fmt.Errorf("validation failed: %w", err)
	}
	
	// Parse review
	review, err := j.parseReview(ctx, &reviewJSON, providerID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse review: %w", err)
	}
	
	// Parse hotel
	hotel, err := j.parseHotel(ctx, &reviewJSON)
	if err != nil {
		j.logger.WarnContext(ctx, "Failed to parse hotel", "error", err)
		hotel = nil
	}
	
	// Parse reviewer info
	reviewer, err := j.parseReviewerInfo(ctx, &reviewJSON)
	if err != nil {
		j.logger.WarnContext(ctx, "Failed to parse reviewer info", "error", err)
		reviewer = nil
	}
	
	return review, hotel, reviewer, nil
}

// parseReview parses a review from JSON
func (j *JSONLinesProcessor) parseReview(ctx context.Context, reviewJSON *ReviewJSON, providerID uuid.UUID) (*domain.Review, error) {
	// Parse rating
	rating, err := j.parseFloat(reviewJSON.Rating)
	if err != nil {
		return nil, fmt.Errorf("invalid rating: %w", err)
	}
	
	// Parse review date
	reviewDate, err := j.parseDate(reviewJSON.ReviewDate)
	if err != nil {
		return nil, fmt.Errorf("invalid review date: %w", err)
	}
	
	// Parse hotel ID
	hotelID, err := uuid.Parse(reviewJSON.HotelID)
	if err != nil {
		// Generate UUID from hotel name if not provided
		hotelID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(reviewJSON.HotelName))
	}
	
	// Parse reviewer ID (generate from email if available)
	reviewerID := uuid.New()
	if reviewJSON.ReviewerEmail != "" {
		reviewerID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(reviewJSON.ReviewerEmail))
	}
	
	review := &domain.Review{
		ID:             uuid.New(),
		ProviderID:     providerID,
		HotelID:        hotelID,
		ReviewerInfoID: &reviewerID,
		ExternalID:     reviewJSON.ID,
		Rating:         rating,
		Title:          reviewJSON.Title,
		Comment:        reviewJSON.Comment,
		ReviewDate:     reviewDate,
		Language:       j.getStringOrDefault(reviewJSON.Language, "en"),
		Source:         reviewJSON.Source,
		Metadata:       reviewJSON.Metadata,
	}
	
	// Parse optional fields
	if reviewJSON.StayDate != "" {
		if stayDate, err := j.parseDate(reviewJSON.StayDate); err == nil {
			review.StayDate = &stayDate
		}
	}
	
	if reviewJSON.TripType != "" {
		review.TripType = reviewJSON.TripType
	}
	
	if reviewJSON.RoomType != "" {
		review.RoomType = reviewJSON.RoomType
	}
	
	if isVerified, err := j.parseBool(reviewJSON.IsVerified); err == nil {
		review.IsVerified = isVerified
	}
	
	if helpfulVotes, err := j.parseInt(reviewJSON.HelpfulVotes); err == nil {
		review.HelpfulVotes = helpfulVotes
	}
	
	if totalVotes, err := j.parseInt(reviewJSON.TotalVotes); err == nil {
		review.TotalVotes = totalVotes
	}
	
	// Parse detailed ratings
	if serviceRating, err := j.parseFloat(reviewJSON.ServiceRating); err == nil {
		review.ServiceRating = &serviceRating
	}
	
	if cleanlinessRating, err := j.parseFloat(reviewJSON.CleanlinessRating); err == nil {
		review.CleanlinessRating = &cleanlinessRating
	}
	
	if locationRating, err := j.parseFloat(reviewJSON.LocationRating); err == nil {
		review.LocationRating = &locationRating
	}
	
	if valueRating, err := j.parseFloat(reviewJSON.ValueRating); err == nil {
		review.ValueRating = &valueRating
	}
	
	if comfortRating, err := j.parseFloat(reviewJSON.ComfortRating); err == nil {
		review.ComfortRating = &comfortRating
	}
	
	if facilitiesRating, err := j.parseFloat(reviewJSON.FacilitiesRating); err == nil {
		review.FacilitiesRating = &facilitiesRating
	}
	
	return review, nil
}

// parseHotel parses hotel information from JSON
func (j *JSONLinesProcessor) parseHotel(ctx context.Context, reviewJSON *ReviewJSON) (*domain.Hotel, error) {
	hotelID, err := uuid.Parse(reviewJSON.HotelID)
	if err != nil {
		hotelID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(reviewJSON.HotelName))
	}
	
	hotel := &domain.Hotel{
		ID:          hotelID,
		Name:        reviewJSON.HotelName,
		Address:     reviewJSON.HotelAddress,
		City:        reviewJSON.HotelCity,
		Country:     reviewJSON.HotelCountry,
		PostalCode:  reviewJSON.HotelPostalCode,
		Phone:       reviewJSON.HotelPhone,
		Email:       reviewJSON.HotelEmail,
		Amenities:   reviewJSON.HotelAmenities,
	}
	
	// Parse optional fields
	if starRating, err := j.parseInt(reviewJSON.HotelStarRating); err == nil {
		hotel.StarRating = starRating
	}
	
	if latitude, err := j.parseFloat(reviewJSON.HotelLatitude); err == nil {
		hotel.Latitude = latitude
	}
	
	if longitude, err := j.parseFloat(reviewJSON.HotelLongitude); err == nil {
		hotel.Longitude = longitude
	}
	
	return hotel, nil
}

// parseReviewerInfo parses reviewer information from JSON
func (j *JSONLinesProcessor) parseReviewerInfo(ctx context.Context, reviewJSON *ReviewJSON) (*domain.ReviewerInfo, error) {
	if reviewJSON.ReviewerEmail == "" {
		return nil, fmt.Errorf("reviewer email is required")
	}
	
	reviewerID := uuid.NewSHA1(uuid.NameSpaceURL, []byte(reviewJSON.ReviewerEmail))
	
	reviewer := &domain.ReviewerInfo{
		ID:      reviewerID,
		Name:     reviewJSON.ReviewerName,
		Email:    reviewJSON.ReviewerEmail,
		Location: reviewJSON.ReviewerCountry,
	}
	
	return reviewer, nil
}

// ValidateFile validates the file format and structure
func (j *JSONLinesProcessor) ValidateFile(ctx context.Context, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	
	// Check first few lines for format validation
	for scanner.Scan() && lineNumber < 10 {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" {
			continue
		}
		
		var reviewJSON ReviewJSON
		if err := json.Unmarshal([]byte(line), &reviewJSON); err != nil {
			return fmt.Errorf("invalid JSON format at line %d: %w", lineNumber, err)
		}
		
		if err := j.validator.validateReviewJSON(&reviewJSON); err != nil {
			return fmt.Errorf("validation failed at line %d: %w", lineNumber, err)
		}
	}
	
	return scanner.Err()
}

// CountRecords counts the number of records in the file
func (j *JSONLinesProcessor) CountRecords(ctx context.Context, reader io.Reader) (int, error) {
	scanner := bufio.NewScanner(reader)
	count := 0
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			count++
		}
	}
	
	return count, scanner.Err()
}

// ParseReview parses a single review from JSON line
func (j *JSONLinesProcessor) ParseReview(ctx context.Context, jsonLine []byte, providerID uuid.UUID) (*domain.Review, error) {
	var reviewJSON ReviewJSON
	if err := json.Unmarshal(jsonLine, &reviewJSON); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	
	if err := j.validator.validateReviewJSON(&reviewJSON); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	
	review, _, _, err := j.parseLine(ctx, string(jsonLine), providerID)
	return review, err
}

// ParseHotel parses hotel information from JSON line
func (j *JSONLinesProcessor) ParseHotel(ctx context.Context, jsonLine []byte) (*domain.Hotel, error) {
	var reviewJSON ReviewJSON
	if err := json.Unmarshal(jsonLine, &reviewJSON); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	
	return j.parseHotel(ctx, &reviewJSON)
}

// ParseReviewerInfo parses reviewer information from JSON line
func (j *JSONLinesProcessor) ParseReviewerInfo(ctx context.Context, jsonLine []byte) (*domain.ReviewerInfo, error) {
	var reviewJSON ReviewJSON
	if err := json.Unmarshal(jsonLine, &reviewJSON); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	
	return j.parseReviewerInfo(ctx, &reviewJSON)
}

// ValidateReview validates a review
func (j *JSONLinesProcessor) ValidateReview(ctx context.Context, review *domain.Review) error {
	if review.Rating < 1.0 || review.Rating > 5.0 {
		return fmt.Errorf("rating must be between 1.0 and 5.0")
	}
	
	if review.Comment == "" {
		return fmt.Errorf("comment cannot be empty")
	}
	
	if len(review.Comment) > j.validator.maxCommentLen {
		return fmt.Errorf("comment exceeds maximum length of %d characters", j.validator.maxCommentLen)
	}
	
	if review.Title != "" && len(review.Title) > j.validator.maxTitleLen {
		return fmt.Errorf("title exceeds maximum length of %d characters", j.validator.maxTitleLen)
	}
	
	return nil
}

// ValidateHotel validates a hotel
func (j *JSONLinesProcessor) ValidateHotel(ctx context.Context, hotel *domain.Hotel) error {
	if hotel.Name == "" {
		return fmt.Errorf("hotel name cannot be empty")
	}
	
	if hotel.StarRating < 1 || hotel.StarRating > 5 {
		return fmt.Errorf("star rating must be between 1 and 5")
	}
	
	return nil
}

// ValidateReviewerInfo validates reviewer information
func (j *JSONLinesProcessor) ValidateReviewerInfo(ctx context.Context, reviewerInfo *domain.ReviewerInfo) error {
	if reviewerInfo.Email == "" {
		return fmt.Errorf("reviewer email cannot be empty")
	}
	
	return nil
}

// ProcessBatch processes a batch of reviews
func (j *JSONLinesProcessor) ProcessBatch(ctx context.Context, reviews []domain.Review) error {
	return j.repository.CreateBatch(ctx, reviews)
}

// GetBatchSize returns the current batch size
func (j *JSONLinesProcessor) GetBatchSize() int {
	return j.batchSize
}

// SetBatchSize sets the batch size
func (j *JSONLinesProcessor) SetBatchSize(size int) {
	j.batchSize = size
}

// Validation methods for ReviewValidator
func (v *ReviewValidator) validateReviewJSON(reviewJSON *ReviewJSON) error {
	for _, field := range v.requiredFields {
		if err := v.checkRequiredField(reviewJSON, field); err != nil {
			return err
		}
	}
	
	return nil
}

func (v *ReviewValidator) checkRequiredField(reviewJSON *ReviewJSON, field string) error {
	switch field {
	case "id":
		if reviewJSON.ID == "" {
			return fmt.Errorf("field '%s' is required", field)
		}
	case "hotel_id":
		if reviewJSON.HotelID == "" {
			return fmt.Errorf("field '%s' is required", field)
		}
	case "hotel_name":
		if reviewJSON.HotelName == "" {
			return fmt.Errorf("field '%s' is required", field)
		}
	case "rating":
		if reviewJSON.Rating == nil {
			return fmt.Errorf("field '%s' is required", field)
		}
	case "comment":
		if reviewJSON.Comment == "" {
			return fmt.Errorf("field '%s' is required", field)
		}
	case "review_date":
		if reviewJSON.ReviewDate == "" {
			return fmt.Errorf("field '%s' is required", field)
		}
	}
	
	return nil
}

// Helper parsing methods
func (j *JSONLinesProcessor) parseFloat(value interface{}) (float64, error) {
	if value == nil {
		return 0, fmt.Errorf("value is nil")
	}
	
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

func (j *JSONLinesProcessor) parseInt(value interface{}) (int, error) {
	if value == nil {
		return 0, fmt.Errorf("value is nil")
	}
	
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case float32:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func (j *JSONLinesProcessor) parseBool(value interface{}) (bool, error) {
	if value == nil {
		return false, fmt.Errorf("value is nil")
	}
	
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

func (j *JSONLinesProcessor) parseDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("date string is empty")
	}
	
	// Try different date formats
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"01/02/2006",
		"02/01/2006",
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("cannot parse date: %s", dateStr)
}

func (j *JSONLinesProcessor) getStringOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}