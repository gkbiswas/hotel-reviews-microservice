# Async Processing Guide

## Overview

This guide covers asynchronous processing patterns in the Hotel Reviews Microservice, including bulk operations, event-driven workflows, and background job processing.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   API Gateway   │    │   Job Queue     │    │   Workers       │
│                 │    │   (Kafka)       │    │                 │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ Request Handler │───▶│ Topic: jobs     │───▶│ Review Processor│
│ Bulk Endpoint   │    │ Partitions: 8   │    │ Image Processor │
│ Job Status API  │    │ Replication: 3  │    │ Email Processor │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │   Job Store     │
                       │   (PostgreSQL)  │
                       │                 │
                       │ - Jobs          │
                       │ - Status        │
                       │ - Results       │
                       └─────────────────┘
```

## Bulk Review Processing

### 1. Submit Bulk Job

```go
// Submit bulk review processing job
func SubmitBulkReviewJob(ctx context.Context, req *BulkJobRequest) (*BulkJobResponse, error) {
    // Create job record
    job := &ProcessingJob{
        ID:          uuid.New(),
        Type:        "bulk_review_import",
        Status:      "queued",
        Payload:     req,
        ScheduledAt: time.Now(),
        MaxRetries:  3,
    }
    
    // Save job to database
    if err := db.CreateJob(ctx, job); err != nil {
        return nil, fmt.Errorf("failed to create job: %w", err)
    }
    
    // Send to job queue
    message := &JobMessage{
        JobID:     job.ID,
        Type:      job.Type,
        Payload:   job.Payload,
        Timestamp: time.Now(),
    }
    
    if err := kafka.Publish(ctx, "jobs", message); err != nil {
        // Update job status to failed
        job.Status = "failed"
        job.ErrorMessage = err.Error()
        db.UpdateJob(ctx, job)
        return nil, fmt.Errorf("failed to queue job: %w", err)
    }
    
    return &BulkJobResponse{
        JobID:     job.ID,
        Status:    job.Status,
        CreatedAt: job.CreatedAt,
    }, nil
}
```

### 2. Process Bulk Reviews

```go
// Worker function for processing bulk reviews
func ProcessBulkReviews(ctx context.Context, jobID uuid.UUID) error {
    // Get job details
    job, err := db.GetJob(ctx, jobID)
    if err != nil {
        return fmt.Errorf("failed to get job: %w", err)
    }
    
    // Update job status to processing
    job.Status = "processing"
    job.StartedAt = time.Now()
    if err := db.UpdateJob(ctx, job); err != nil {
        return fmt.Errorf("failed to update job status: %w", err)
    }
    
    // Parse job payload
    var request BulkJobRequest
    if err := json.Unmarshal(job.Payload, &request); err != nil {
        return fmt.Errorf("failed to parse job payload: %w", err)
    }
    
    // Download and process file
    processor := NewBulkProcessor(request.ProviderID, request.FileURL)
    result, err := processor.Process(ctx, func(progress Progress) {
        // Update job progress
        job.Result = map[string]interface{}{
            "progress": progress,
        }
        db.UpdateJob(ctx, job)
    })
    
    // Update final job status
    if err != nil {
        job.Status = "failed"
        job.ErrorMessage = err.Error()
    } else {
        job.Status = "completed"
        job.Result = result
    }
    job.CompletedAt = time.Now()
    
    if err := db.UpdateJob(ctx, job); err != nil {
        log.Errorf("Failed to update job completion: %v", err)
    }
    
    // Send webhook notification if configured
    if request.CallbackURL != "" {
        go sendWebhookNotification(request.CallbackURL, job)
    }
    
    return err
}
```

### 3. Bulk File Processor

```go
type BulkProcessor struct {
    providerID uuid.UUID
    fileURL    string
    batchSize  int
    workers    int
}

func NewBulkProcessor(providerID uuid.UUID, fileURL string) *BulkProcessor {
    return &BulkProcessor{
        providerID: providerID,
        fileURL:    fileURL,
        batchSize:  100,
        workers:    8,
    }
}

func (bp *BulkProcessor) Process(ctx context.Context, progressCallback func(Progress)) (*ProcessingResult, error) {
    // Download file
    reader, err := bp.downloadFile(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to download file: %w", err)
    }
    defer reader.Close()
    
    // Create worker pool
    jobs := make(chan []Review, bp.workers*2)
    results := make(chan BatchResult, bp.workers*2)
    
    var wg sync.WaitGroup
    for i := 0; i < bp.workers; i++ {
        wg.Add(1)
        go bp.worker(ctx, jobs, results, &wg)
    }
    
    // Process file in batches
    go func() {
        defer close(jobs)
        defer wg.Wait()
        defer close(results)
        
        scanner := bufio.NewScanner(reader)
        batch := make([]Review, 0, bp.batchSize)
        
        for scanner.Scan() {
            var review Review
            if err := json.Unmarshal(scanner.Bytes(), &review); err != nil {
                log.Warnf("Failed to parse review: %v", err)
                continue
            }
            
            batch = append(batch, review)
            if len(batch) >= bp.batchSize {
                jobs <- batch
                batch = make([]Review, 0, bp.batchSize)
            }
        }
        
        // Send remaining batch
        if len(batch) > 0 {
            jobs <- batch
        }
    }()
    
    // Collect results
    result := &ProcessingResult{
        StartTime: time.Now(),
    }
    
    for batchResult := range results {
        result.TotalProcessed += batchResult.Processed
        result.TotalFailed += batchResult.Failed
        result.Errors = append(result.Errors, batchResult.Errors...)
        
        // Report progress
        if progressCallback != nil {
            progress := Progress{
                Total:      result.TotalProcessed + result.TotalFailed,
                Processed:  result.TotalProcessed,
                Failed:     result.TotalFailed,
                Percentage: float64(result.TotalProcessed) / float64(result.TotalProcessed+result.TotalFailed) * 100,
            }
            progressCallback(progress)
        }
    }
    
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    
    return result, nil
}

func (bp *BulkProcessor) worker(ctx context.Context, jobs <-chan []Review, results chan<- BatchResult, wg *sync.WaitGroup) {
    defer wg.Done()
    
    for batch := range jobs {
        result := BatchResult{}
        
        for _, review := range batch {
            // Validate review
            if err := validateReview(&review); err != nil {
                result.Failed++
                result.Errors = append(result.Errors, err.Error())
                continue
            }
            
            // Set provider ID
            review.ProviderID = bp.providerID
            
            // Save review
            if err := db.CreateReview(ctx, &review); err != nil {
                result.Failed++
                result.Errors = append(result.Errors, err.Error())
                continue
            }
            
            result.Processed++
        }
        
        results <- result
    }
}
```

## Event-Driven Processing

### 1. Review Events

```go
// Event types
const (
    EventReviewCreated = "review.created"
    EventReviewUpdated = "review.updated"
    EventReviewDeleted = "review.deleted"
)

// Event structure
type ReviewEvent struct {
    Type        string    `json:"type"`
    ReviewID    uuid.UUID `json:"review_id"`
    HotelID     uuid.UUID `json:"hotel_id"`
    ProviderID  uuid.UUID `json:"provider_id"`
    Timestamp   time.Time `json:"timestamp"`
    Data        Review    `json:"data"`
    Metadata    map[string]interface{} `json:"metadata"`
}

// Publish review events
func PublishReviewEvent(ctx context.Context, eventType string, review *Review) error {
    event := &ReviewEvent{
        Type:       eventType,
        ReviewID:   review.ID,
        HotelID:    review.HotelID,
        ProviderID: review.ProviderID,
        Timestamp:  time.Now(),
        Data:       *review,
        Metadata: map[string]interface{}{
            "source":     "api",
            "user_agent": getUserAgent(ctx),
            "ip_address": getClientIP(ctx),
        },
    }
    
    return kafka.Publish(ctx, "review-events", event)
}
```

### 2. Event Handlers

```go
// Hotel rating update handler
func HandleReviewEvents(ctx context.Context, event *ReviewEvent) error {
    switch event.Type {
    case EventReviewCreated, EventReviewUpdated, EventReviewDeleted:
        return updateHotelRating(ctx, event.HotelID)
    default:
        log.Warnf("Unknown event type: %s", event.Type)
        return nil
    }
}

func updateHotelRating(ctx context.Context, hotelID uuid.UUID) error {
    // Calculate new average rating
    stats, err := db.GetHotelReviewStats(ctx, hotelID)
    if err != nil {
        return fmt.Errorf("failed to get hotel stats: %w", err)
    }
    
    // Update hotel rating
    if err := db.UpdateHotelRating(ctx, hotelID, stats.AverageRating, stats.ReviewCount); err != nil {
        return fmt.Errorf("failed to update hotel rating: %w", err)
    }
    
    // Invalidate cache
    cacheService.InvalidatePattern(ctx, fmt.Sprintf("hotel:%s:*", hotelID))
    
    // Send hotel updated event
    hotelEvent := &HotelEvent{
        Type:      "hotel.rating_updated",
        HotelID:   hotelID,
        Timestamp: time.Now(),
        Data: map[string]interface{}{
            "new_rating": stats.AverageRating,
            "review_count": stats.ReviewCount,
        },
    }
    
    return kafka.Publish(ctx, "hotel-events", hotelEvent)
}
```

### 3. Sentiment Analysis Pipeline

```go
// Async sentiment analysis
func ProcessSentimentAnalysis(ctx context.Context, reviewID uuid.UUID) error {
    review, err := db.GetReview(ctx, reviewID)
    if err != nil {
        return fmt.Errorf("failed to get review: %w", err)
    }
    
    // Call sentiment analysis service
    sentiment, err := sentimentService.Analyze(ctx, review.Content, review.Language)
    if err != nil {
        return fmt.Errorf("sentiment analysis failed: %w", err)
    }
    
    // Update review with sentiment score
    review.SentimentScore = sentiment.Score
    if err := db.UpdateReview(ctx, review); err != nil {
        return fmt.Errorf("failed to update sentiment: %w", err)
    }
    
    // Send sentiment analyzed event
    event := &SentimentEvent{
        Type:           "review.sentiment_analyzed",
        ReviewID:       reviewID,
        SentimentScore: sentiment.Score,
        Confidence:     sentiment.Confidence,
        Timestamp:      time.Now(),
    }
    
    return kafka.Publish(ctx, "sentiment-events", event)
}
```

## Image Processing

### 1. Image Upload Processing

```go
func ProcessImageUploads(ctx context.Context, reviewID uuid.UUID, imageURLs []string) error {
    for _, imageURL := range imageURLs {
        // Create async job for each image
        job := &ImageProcessingJob{
            ReviewID: reviewID,
            ImageURL: imageURL,
            Tasks: []string{
                "resize",
                "thumbnail",
                "moderate",
                "extract_metadata",
            },
        }
        
        if err := submitImageProcessingJob(ctx, job); err != nil {
            log.Errorf("Failed to submit image processing job: %v", err)
        }
    }
    
    return nil
}

func processImage(ctx context.Context, job *ImageProcessingJob) error {
    // Download original image
    image, err := downloadImage(ctx, job.ImageURL)
    if err != nil {
        return fmt.Errorf("failed to download image: %w", err)
    }
    
    results := make(map[string]interface{})
    
    // Process each task
    for _, task := range job.Tasks {
        switch task {
        case "resize":
            resized, err := resizeImage(image, 1200, 800)
            if err != nil {
                log.Errorf("Failed to resize image: %v", err)
                continue
            }
            url, _ := uploadToS3(ctx, resized, "resized")
            results["resized_url"] = url
            
        case "thumbnail":
            thumbnail, err := createThumbnail(image, 300, 200)
            if err != nil {
                log.Errorf("Failed to create thumbnail: %v", err)
                continue
            }
            url, _ := uploadToS3(ctx, thumbnail, "thumbnail")
            results["thumbnail_url"] = url
            
        case "moderate":
            moderation, err := moderateImage(ctx, image)
            if err != nil {
                log.Errorf("Failed to moderate image: %v", err)
                continue
            }
            results["moderation"] = moderation
            
        case "extract_metadata":
            metadata, err := extractImageMetadata(image)
            if err != nil {
                log.Errorf("Failed to extract metadata: %v", err)
                continue
            }
            results["metadata"] = metadata
        }
    }
    
    // Save results to database
    imageRecord := &ReviewImage{
        ReviewID:     job.ReviewID,
        OriginalURL:  job.ImageURL,
        ResizedURL:   results["resized_url"].(string),
        ThumbnailURL: results["thumbnail_url"].(string),
        Metadata:     results["metadata"],
        Moderation:   results["moderation"],
        ProcessedAt:  time.Now(),
    }
    
    return db.CreateReviewImage(ctx, imageRecord)
}
```

## Email Notifications

### 1. Email Processing Pipeline

```go
type EmailJob struct {
    Type        string                 `json:"type"`
    Recipients  []string               `json:"recipients"`
    Template    string                 `json:"template"`
    Data        map[string]interface{} `json:"data"`
    Priority    string                 `json:"priority"`
    ScheduledAt time.Time              `json:"scheduled_at"`
}

func ProcessEmailNotifications(ctx context.Context, job *EmailJob) error {
    // Load email template
    template, err := emailTemplateService.Get(job.Template)
    if err != nil {
        return fmt.Errorf("failed to load template: %w", err)
    }
    
    // Render email content
    content, err := template.Render(job.Data)
    if err != nil {
        return fmt.Errorf("failed to render template: %w", err)
    }
    
    // Send emails
    var errors []error
    for _, recipient := range job.Recipients {
        email := &Email{
            To:      recipient,
            Subject: content.Subject,
            Body:    content.Body,
            IsHTML:  true,
        }
        
        if err := emailService.Send(ctx, email); err != nil {
            errors = append(errors, fmt.Errorf("failed to send to %s: %w", recipient, err))
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("email sending errors: %v", errors)
    }
    
    return nil
}

// Trigger email notifications for events
func HandleEmailNotifications(ctx context.Context, event interface{}) error {
    switch e := event.(type) {
    case *ReviewEvent:
        if e.Type == EventReviewCreated {
            return sendNewReviewNotification(ctx, &e.Data)
        }
    case *BulkJobEvent:
        if e.Type == "bulk_job.completed" {
            return sendBulkJobCompletionNotification(ctx, e)
        }
    }
    
    return nil
}
```

## Job Monitoring and Recovery

### 1. Job Status Tracking

```go
func GetJobStatus(ctx context.Context, jobID uuid.UUID) (*JobStatus, error) {
    job, err := db.GetJob(ctx, jobID)
    if err != nil {
        return nil, fmt.Errorf("job not found: %w", err)
    }
    
    status := &JobStatus{
        JobID:     job.ID,
        Type:      job.Type,
        Status:    job.Status,
        CreatedAt: job.CreatedAt,
        StartedAt: job.StartedAt,
        CompletedAt: job.CompletedAt,
    }
    
    // Add progress information if available
    if job.Result != nil {
        if progress, ok := job.Result["progress"]; ok {
            status.Progress = progress.(Progress)
        }
    }
    
    // Calculate estimated completion time
    if job.Status == "processing" && job.StartedAt != nil {
        elapsed := time.Since(*job.StartedAt)
        if status.Progress.Percentage > 0 {
            estimatedTotal := elapsed * time.Duration(100/status.Progress.Percentage)
            estimatedCompletion := job.StartedAt.Add(estimatedTotal)
            status.EstimatedCompletion = &estimatedCompletion
        }
    }
    
    return status, nil
}
```

### 2. Failed Job Recovery

```go
// Retry failed jobs
func RetryFailedJobs(ctx context.Context) error {
    failedJobs, err := db.GetFailedJobs(ctx, time.Hour*24) // Jobs failed in last 24 hours
    if err != nil {
        return fmt.Errorf("failed to get failed jobs: %w", err)
    }
    
    for _, job := range failedJobs {
        if job.RetryCount >= job.MaxRetries {
            continue // Skip jobs that have exceeded max retries
        }
        
        // Increment retry count
        job.RetryCount++
        job.Status = "queued"
        job.ErrorMessage = ""
        job.ScheduledAt = time.Now().Add(time.Duration(job.RetryCount) * time.Minute) // Exponential backoff
        
        if err := db.UpdateJob(ctx, job); err != nil {
            log.Errorf("Failed to update job for retry: %v", err)
            continue
        }
        
        // Requeue job
        message := &JobMessage{
            JobID:     job.ID,
            Type:      job.Type,
            Payload:   job.Payload,
            Timestamp: time.Now(),
            Retry:     true,
        }
        
        if err := kafka.Publish(ctx, "jobs", message); err != nil {
            log.Errorf("Failed to requeue job: %v", err)
        }
    }
    
    return nil
}
```

### 3. Dead Letter Queue

```go
// Handle jobs that have failed all retries
func ProcessDeadLetterQueue(ctx context.Context) error {
    deadJobs, err := db.GetDeadJobs(ctx)
    if err != nil {
        return fmt.Errorf("failed to get dead jobs: %w", err)
    }
    
    for _, job := range deadJobs {
        // Log dead job for investigation
        log.Errorf("Dead job detected: ID=%s, Type=%s, Error=%s", 
            job.ID, job.Type, job.ErrorMessage)
        
        // Send alert to operations team
        alert := &Alert{
            Type:        "dead_job",
            Severity:    "high",
            Message:     fmt.Sprintf("Job %s has failed all retries", job.ID),
            Details:     job,
            Timestamp:   time.Now(),
        }
        
        alertService.Send(ctx, alert)
        
        // Move to dead letter storage
        if err := archiveDeadJob(ctx, job); err != nil {
            log.Errorf("Failed to archive dead job: %v", err)
        }
    }
    
    return nil
}
```

## Performance Optimization

### 1. Batch Processing

```go
// Optimize database operations with batching
func ProcessReviewsBatch(ctx context.Context, reviews []Review) error {
    batchSize := 100
    
    for i := 0; i < len(reviews); i += batchSize {
        end := i + batchSize
        if end > len(reviews) {
            end = len(reviews)
        }
        
        batch := reviews[i:end]
        
        // Use transaction for batch
        tx, err := db.BeginTx(ctx, nil)
        if err != nil {
            return fmt.Errorf("failed to begin transaction: %w", err)
        }
        
        for _, review := range batch {
            if err := db.CreateReviewTx(ctx, tx, &review); err != nil {
                tx.Rollback()
                return fmt.Errorf("failed to create review in batch: %w", err)
            }
        }
        
        if err := tx.Commit(); err != nil {
            return fmt.Errorf("failed to commit batch: %w", err)
        }
        
        // Rate limiting between batches
        time.Sleep(50 * time.Millisecond)
    }
    
    return nil
}
```

### 2. Parallel Processing

```go
// Process multiple jobs in parallel with controlled concurrency
func ProcessJobsConcurrently(ctx context.Context, jobs []ProcessingJob, maxWorkers int) error {
    jobChan := make(chan ProcessingJob, len(jobs))
    resultChan := make(chan error, len(jobs))
    
    // Start workers
    for i := 0; i < maxWorkers; i++ {
        go func() {
            for job := range jobChan {
                err := processJob(ctx, job)
                resultChan <- err
            }
        }()
    }
    
    // Send jobs to workers
    go func() {
        defer close(jobChan)
        for _, job := range jobs {
            jobChan <- job
        }
    }()
    
    // Collect results
    var errors []error
    for i := 0; i < len(jobs); i++ {
        if err := <-resultChan; err != nil {
            errors = append(errors, err)
        }
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("job processing errors: %v", errors)
    }
    
    return nil
}
```

This comprehensive guide covers all aspects of asynchronous processing in the Hotel Reviews Microservice, from bulk operations to event-driven workflows and performance optimization strategies.