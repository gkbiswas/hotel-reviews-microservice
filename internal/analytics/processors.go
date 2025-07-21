package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// HotelEventProcessor processes hotel-related events
type HotelEventProcessor struct {
	redisClient RedisClient
	logger      *slog.Logger
}

// NewHotelEventProcessor creates a new hotel event processor
func NewHotelEventProcessor(redisClient RedisClient, logger *slog.Logger) *HotelEventProcessor {
	return &HotelEventProcessor{
		redisClient: redisClient,
		logger:      logger,
	}
}

// Process processes hotel events and generates metrics
func (p *HotelEventProcessor) Process(ctx context.Context, event *AnalyticsEvent) (*ProcessedEvent, error) {
	processed := &ProcessedEvent{
		OriginalEvent: event,
		Metrics:       []Metric{},
		Aggregations:  []Aggregation{},
		Enrichments:   make(map[string]interface{}),
		Timestamp:     time.Now().UTC(),
	}

	switch event.Type {
	case "hotel_view":
		return p.processHotelView(ctx, event, processed)
	case "hotel_search":
		return p.processHotelSearch(ctx, event, processed)
	case "hotel_booking_attempt":
		return p.processBookingAttempt(ctx, event, processed)
	case "hotel_created":
		return p.processHotelCreated(ctx, event, processed)
	case "hotel_updated":
		return p.processHotelUpdated(ctx, event, processed)
	default:
		return processed, nil
	}
}

// GetEventTypes returns the event types this processor handles
func (p *HotelEventProcessor) GetEventTypes() []string {
	return []string{
		"hotel_view",
		"hotel_search",
		"hotel_booking_attempt",
		"hotel_created",
		"hotel_updated",
	}
}

func (p *HotelEventProcessor) processHotelView(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	if event.HotelID == "" {
		return processed, fmt.Errorf("hotel_id is required for hotel_view event")
	}

	// Track hotel popularity
	viewsKey := fmt.Sprintf("hotel_views:%s", event.HotelID)
	views, err := p.redisClient.Incr(ctx, viewsKey)
	if err != nil {
		return processed, fmt.Errorf("failed to increment hotel views: %w", err)
	}
	p.redisClient.Expire(ctx, viewsKey, 24*time.Hour)

	// Add metrics
	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "hotel_view_count",
		Value:     float64(views),
		Unit:      "count",
		Tags:      map[string]string{"hotel_id": event.HotelID},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	// Track geographical distribution
	if event.GeoLocation != nil {
		countryKey := fmt.Sprintf("hotel_views_by_country:%s:%s", event.HotelID, event.GeoLocation.Country)
		countryViews, _ := p.redisClient.Incr(ctx, countryKey)
		p.redisClient.Expire(ctx, countryKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:  "hotel_views_by_country",
			Value: float64(countryViews),
			Unit:  "count",
			Tags: map[string]string{
				"hotel_id": event.HotelID,
				"country":  event.GeoLocation.Country,
			},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	// Track device type distribution
	if event.DeviceInfo != nil {
		deviceKey := fmt.Sprintf("hotel_views_by_device:%s:%s", event.HotelID, event.DeviceInfo.Type)
		deviceViews, _ := p.redisClient.Incr(ctx, deviceKey)
		p.redisClient.Expire(ctx, deviceKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:  "hotel_views_by_device",
			Value: float64(deviceViews),
			Unit:  "count",
			Tags: map[string]string{
				"hotel_id":    event.HotelID,
				"device_type": event.DeviceInfo.Type,
			},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	// Add enrichments
	processed.Enrichments["popularity_score"] = p.calculatePopularityScore(views)
	processed.Enrichments["view_velocity"] = p.calculateViewVelocity(ctx, event.HotelID)

	return processed, nil
}

func (p *HotelEventProcessor) processHotelSearch(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Extract search parameters
	query, _ := event.Properties["query"].(string)
	checkIn, _ := event.Properties["check_in"].(string)
	checkOut, _ := event.Properties["check_out"].(string)
	guests, _ := event.Properties["guests"].(float64)
	location, _ := event.Properties["location"].(string)

	// Track search frequency
	searchKey := "hotel_searches_total"
	searches, _ := p.redisClient.Incr(ctx, searchKey)
	p.redisClient.Expire(ctx, searchKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "hotel_search_count",
		Value:     float64(searches),
		Unit:      "count",
		Tags:      map[string]string{"location": location},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	// Track popular search terms
	if query != "" {
		queryKey := fmt.Sprintf("search_terms:%s", strings.ToLower(query))
		queryCount, _ := p.redisClient.Incr(ctx, queryKey)
		p.redisClient.Expire(ctx, queryKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "search_term_frequency",
			Value:     float64(queryCount),
			Unit:      "count",
			Tags:      map[string]string{"query": query},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	// Track booking patterns
	if checkIn != "" && checkOut != "" {
		checkInTime, err1 := time.Parse("2006-01-02", checkIn)
		checkOutTime, err2 := time.Parse("2006-01-02", checkOut)
		
		if err1 == nil && err2 == nil {
			duration := checkOutTime.Sub(checkInTime).Hours() / 24
			durationKey := fmt.Sprintf("booking_duration:%d", int(duration))
			durationCount, _ := p.redisClient.Incr(ctx, durationKey)
			p.redisClient.Expire(ctx, durationKey, 24*time.Hour)

			processed.Metrics = append(processed.Metrics, Metric{
				Name:      "booking_duration_frequency",
				Value:     float64(durationCount),
				Unit:      "count",
				Tags:      map[string]string{"duration_days": fmt.Sprintf("%.0f", duration)},
				Timestamp: event.Timestamp,
				TTL:       24 * time.Hour,
			})
		}
	}

	// Track guest count distribution
	if guests > 0 {
		guestsKey := fmt.Sprintf("guest_count:%d", int(guests))
		guestCount, _ := p.redisClient.Incr(ctx, guestsKey)
		p.redisClient.Expire(ctx, guestsKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "guest_count_frequency",
			Value:     float64(guestCount),
			Unit:      "count",
			Tags:      map[string]string{"guests": fmt.Sprintf("%.0f", guests)},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	return processed, nil
}

func (p *HotelEventProcessor) processBookingAttempt(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	if event.HotelID == "" {
		return processed, fmt.Errorf("hotel_id is required for booking_attempt event")
	}

	// Track booking attempts
	attemptKey := fmt.Sprintf("booking_attempts:%s", event.HotelID)
	attempts, _ := p.redisClient.Incr(ctx, attemptKey)
	p.redisClient.Expire(ctx, attemptKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "booking_attempt_count",
		Value:     float64(attempts),
		Unit:      "count",
		Tags:      map[string]string{"hotel_id": event.HotelID},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	// Track conversion funnel
	viewsKey := fmt.Sprintf("hotel_views:%s", event.HotelID)
	views, _ := p.redisClient.Get(ctx, viewsKey)
	if views != "" {
		viewCount, _ := strconv.ParseInt(views, 10, 64)
		if viewCount > 0 {
			conversionRate := float64(attempts) / float64(viewCount) * 100
			processed.Metrics = append(processed.Metrics, Metric{
				Name:      "view_to_booking_conversion_rate",
				Value:     conversionRate,
				Unit:      "percent",
				Tags:      map[string]string{"hotel_id": event.HotelID},
				Timestamp: event.Timestamp,
				TTL:       24 * time.Hour,
			})
		}
	}

	return processed, nil
}

func (p *HotelEventProcessor) processHotelCreated(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track new hotel registrations
	newHotelsKey := "new_hotels_count"
	newHotels, _ := p.redisClient.Incr(ctx, newHotelsKey)
	p.redisClient.Expire(ctx, newHotelsKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "new_hotels_count",
		Value:     float64(newHotels),
		Unit:      "count",
		Tags:      map[string]string{},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	return processed, nil
}

func (p *HotelEventProcessor) processHotelUpdated(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track hotel profile updates
	updatesKey := fmt.Sprintf("hotel_updates:%s", event.HotelID)
	updates, _ := p.redisClient.Incr(ctx, updatesKey)
	p.redisClient.Expire(ctx, updatesKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "hotel_update_count",
		Value:     float64(updates),
		Unit:      "count",
		Tags:      map[string]string{"hotel_id": event.HotelID},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	return processed, nil
}

func (p *HotelEventProcessor) calculatePopularityScore(views int64) float64 {
	// Simple popularity score based on views (could be more sophisticated)
	return float64(views) / 100.0
}

func (p *HotelEventProcessor) calculateViewVelocity(ctx context.Context, hotelID string) float64 {
	// Calculate views per hour for the last 24 hours
	totalViews := int64(0)
	for i := 0; i < 24; i++ {
		hourKey := fmt.Sprintf("hotel_views_hourly:%s:%s", hotelID, 
			time.Now().Add(-time.Duration(i)*time.Hour).Format("2006010215"))
		views, _ := p.redisClient.Get(ctx, hourKey)
		if views != "" {
			hourlyViews, _ := strconv.ParseInt(views, 10, 64)
			totalViews += hourlyViews
		}
	}
	return float64(totalViews) / 24.0
}

// ReviewEventProcessor processes review-related events
type ReviewEventProcessor struct {
	redisClient RedisClient
	logger      *slog.Logger
}

// NewReviewEventProcessor creates a new review event processor
func NewReviewEventProcessor(redisClient RedisClient, logger *slog.Logger) *ReviewEventProcessor {
	return &ReviewEventProcessor{
		redisClient: redisClient,
		logger:      logger,
	}
}

// Process processes review events and generates metrics
func (p *ReviewEventProcessor) Process(ctx context.Context, event *AnalyticsEvent) (*ProcessedEvent, error) {
	processed := &ProcessedEvent{
		OriginalEvent: event,
		Metrics:       []Metric{},
		Aggregations:  []Aggregation{},
		Enrichments:   make(map[string]interface{}),
		Timestamp:     time.Now().UTC(),
	}

	switch event.Type {
	case "review_created":
		return p.processReviewCreated(ctx, event, processed)
	case "review_updated":
		return p.processReviewUpdated(ctx, event, processed)
	case "review_liked":
		return p.processReviewLiked(ctx, event, processed)
	case "review_reported":
		return p.processReviewReported(ctx, event, processed)
	default:
		return processed, nil
	}
}

// GetEventTypes returns the event types this processor handles
func (p *ReviewEventProcessor) GetEventTypes() []string {
	return []string{
		"review_created",
		"review_updated",
		"review_liked",
		"review_reported",
	}
}

func (p *ReviewEventProcessor) processReviewCreated(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	if event.HotelID == "" || event.ReviewID == "" {
		return processed, fmt.Errorf("hotel_id and review_id are required for review_created event")
	}

	rating, _ := event.Properties["rating"].(float64)
	sentiment, _ := event.Properties["sentiment"].(string)

	// Track review count
	reviewsKey := fmt.Sprintf("hotel_reviews:%s", event.HotelID)
	reviews, _ := p.redisClient.Incr(ctx, reviewsKey)
	p.redisClient.Expire(ctx, reviewsKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "review_count",
		Value:     float64(reviews),
		Unit:      "count",
		Tags:      map[string]string{"hotel_id": event.HotelID},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	// Track rating distribution
	if rating > 0 {
		ratingKey := fmt.Sprintf("rating_distribution:%s:%d", event.HotelID, int(rating))
		ratingCount, _ := p.redisClient.Incr(ctx, ratingKey)
		p.redisClient.Expire(ctx, ratingKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "rating_distribution",
			Value:     float64(ratingCount),
			Unit:      "count",
			Tags: map[string]string{
				"hotel_id": event.HotelID,
				"rating":   fmt.Sprintf("%.0f", rating),
			},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})

		// Update running average
		p.updateRunningAverage(ctx, event.HotelID, rating)
	}

	// Track sentiment distribution
	if sentiment != "" {
		sentimentKey := fmt.Sprintf("sentiment_distribution:%s:%s", event.HotelID, sentiment)
		sentimentCount, _ := p.redisClient.Incr(ctx, sentimentKey)
		p.redisClient.Expire(ctx, sentimentKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "sentiment_distribution",
			Value:     float64(sentimentCount),
			Unit:      "count",
			Tags: map[string]string{
				"hotel_id":  event.HotelID,
				"sentiment": sentiment,
			},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	return processed, nil
}

func (p *ReviewEventProcessor) processReviewUpdated(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track review updates
	updatesKey := fmt.Sprintf("review_updates:%s", event.ReviewID)
	updates, _ := p.redisClient.Incr(ctx, updatesKey)
	p.redisClient.Expire(ctx, updatesKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "review_update_count",
		Value:     float64(updates),
		Unit:      "count",
		Tags:      map[string]string{"review_id": event.ReviewID},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	return processed, nil
}

func (p *ReviewEventProcessor) processReviewLiked(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track review likes
	likesKey := fmt.Sprintf("review_likes:%s", event.ReviewID)
	likes, _ := p.redisClient.Incr(ctx, likesKey)
	p.redisClient.Expire(ctx, likesKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "review_like_count",
		Value:     float64(likes),
		Unit:      "count",
		Tags:      map[string]string{"review_id": event.ReviewID},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	return processed, nil
}

func (p *ReviewEventProcessor) processReviewReported(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track review reports
	reportsKey := fmt.Sprintf("review_reports:%s", event.ReviewID)
	reports, _ := p.redisClient.Incr(ctx, reportsKey)
	p.redisClient.Expire(ctx, reportsKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "review_report_count",
		Value:     float64(reports),
		Unit:      "count",
		Tags:      map[string]string{"review_id": event.ReviewID},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	return processed, nil
}

func (p *ReviewEventProcessor) updateRunningAverage(ctx context.Context, hotelID string, newRating float64) {
	// Update running average rating
	avgKey := fmt.Sprintf("hotel_rating_avg:%s", hotelID)
	countKey := fmt.Sprintf("hotel_rating_count:%s", hotelID)

	// Get current average and count
	avgStr, _ := p.redisClient.Get(ctx, avgKey)
	countStr, _ := p.redisClient.Get(ctx, countKey)

	currentAvg, _ := strconv.ParseFloat(avgStr, 64)
	currentCount, _ := strconv.ParseInt(countStr, 10, 64)

	// Calculate new average
	newCount := currentCount + 1
	newAvg := (currentAvg*float64(currentCount) + newRating) / float64(newCount)

	// Store updated values
	p.redisClient.Set(ctx, avgKey, newAvg, 24*time.Hour)
	p.redisClient.Set(ctx, countKey, newCount, 24*time.Hour)
}

// UserEventProcessor processes user-related events
type UserEventProcessor struct {
	redisClient RedisClient
	logger      *slog.Logger
}

// NewUserEventProcessor creates a new user event processor
func NewUserEventProcessor(redisClient RedisClient, logger *slog.Logger) *UserEventProcessor {
	return &UserEventProcessor{
		redisClient: redisClient,
		logger:      logger,
	}
}

// Process processes user events and generates metrics
func (p *UserEventProcessor) Process(ctx context.Context, event *AnalyticsEvent) (*ProcessedEvent, error) {
	processed := &ProcessedEvent{
		OriginalEvent: event,
		Metrics:       []Metric{},
		Aggregations:  []Aggregation{},
		Enrichments:   make(map[string]interface{}),
		Timestamp:     time.Now().UTC(),
	}

	switch event.Type {
	case "user_signup":
		return p.processUserSignup(ctx, event, processed)
	case "user_login":
		return p.processUserLogin(ctx, event, processed)
	case "user_logout":
		return p.processUserLogout(ctx, event, processed)
	case "user_profile_updated":
		return p.processProfileUpdated(ctx, event, processed)
	default:
		return processed, nil
	}
}

// GetEventTypes returns the event types this processor handles
func (p *UserEventProcessor) GetEventTypes() []string {
	return []string{
		"user_signup",
		"user_login",
		"user_logout",
		"user_profile_updated",
	}
}

func (p *UserEventProcessor) processUserSignup(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track new user signups
	signupsKey := "user_signups_count"
	signups, _ := p.redisClient.Incr(ctx, signupsKey)
	p.redisClient.Expire(ctx, signupsKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "user_signup_count",
		Value:     float64(signups),
		Unit:      "count",
		Tags:      map[string]string{},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	// Track signup source
	source, _ := event.Properties["source"].(string)
	if source != "" {
		sourceKey := fmt.Sprintf("signup_source:%s", source)
		sourceCount, _ := p.redisClient.Incr(ctx, sourceKey)
		p.redisClient.Expire(ctx, sourceKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "signup_by_source",
			Value:     float64(sourceCount),
			Unit:      "count",
			Tags:      map[string]string{"source": source},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	return processed, nil
}

func (p *UserEventProcessor) processUserLogin(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track daily active users
	dauKey := fmt.Sprintf("dau:%s", event.Timestamp.Format("20060102"))
	p.redisClient.ZIncr(ctx, dauKey, event.UserID, 1)
	p.redisClient.Expire(ctx, dauKey, 25*time.Hour)

	// Track weekly active users
	wauKey := fmt.Sprintf("wau:%s", event.Timestamp.Format("2006-W01"))
	p.redisClient.ZIncr(ctx, wauKey, event.UserID, 1)
	p.redisClient.Expire(ctx, wauKey, 8*24*time.Hour)

	// Track monthly active users
	mauKey := fmt.Sprintf("mau:%s", event.Timestamp.Format("200601"))
	p.redisClient.ZIncr(ctx, mauKey, event.UserID, 1)
	p.redisClient.Expire(ctx, mauKey, 32*24*time.Hour)

	return processed, nil
}

func (p *UserEventProcessor) processUserLogout(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track session duration if available
	sessionStart, _ := event.Properties["session_start"].(string)
	if sessionStart != "" {
		startTime, err := time.Parse(time.RFC3339, sessionStart)
		if err == nil {
			duration := event.Timestamp.Sub(startTime)
			
			durationKey := "session_durations"
			p.redisClient.ZIncr(ctx, durationKey, fmt.Sprintf("%.0f", duration.Minutes()), 1)
			p.redisClient.Expire(ctx, durationKey, 24*time.Hour)

			processed.Metrics = append(processed.Metrics, Metric{
				Name:      "session_duration",
				Value:     duration.Minutes(),
				Unit:      "minutes",
				Tags:      map[string]string{"user_id": event.UserID},
				Timestamp: event.Timestamp,
				TTL:       24 * time.Hour,
			})
		}
	}

	return processed, nil
}

func (p *UserEventProcessor) processProfileUpdated(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	// Track profile updates
	updatesKey := fmt.Sprintf("profile_updates:%s", event.UserID)
	updates, _ := p.redisClient.Incr(ctx, updatesKey)
	p.redisClient.Expire(ctx, updatesKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "profile_update_count",
		Value:     float64(updates),
		Unit:      "count",
		Tags:      map[string]string{"user_id": event.UserID},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	return processed, nil
}

// SearchEventProcessor processes search-related events
type SearchEventProcessor struct {
	redisClient RedisClient
	logger      *slog.Logger
}

// NewSearchEventProcessor creates a new search event processor
func NewSearchEventProcessor(redisClient RedisClient, logger *slog.Logger) *SearchEventProcessor {
	return &SearchEventProcessor{
		redisClient: redisClient,
		logger:      logger,
	}
}

// Process processes search events and generates metrics
func (p *SearchEventProcessor) Process(ctx context.Context, event *AnalyticsEvent) (*ProcessedEvent, error) {
	processed := &ProcessedEvent{
		OriginalEvent: event,
		Metrics:       []Metric{},
		Aggregations:  []Aggregation{},
		Enrichments:   make(map[string]interface{}),
		Timestamp:     time.Now().UTC(),
	}

	switch event.Type {
	case "search_performed":
		return p.processSearchPerformed(ctx, event, processed)
	case "search_result_clicked":
		return p.processSearchResultClicked(ctx, event, processed)
	case "search_filter_applied":
		return p.processSearchFilterApplied(ctx, event, processed)
	default:
		return processed, nil
	}
}

// GetEventTypes returns the event types this processor handles
func (p *SearchEventProcessor) GetEventTypes() []string {
	return []string{
		"search_performed",
		"search_result_clicked",
		"search_filter_applied",
	}
}

func (p *SearchEventProcessor) processSearchPerformed(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	query, _ := event.Properties["query"].(string)
	resultsCount, _ := event.Properties["results_count"].(float64)

	// Track total searches
	searchesKey := "total_searches"
	searches, _ := p.redisClient.Incr(ctx, searchesKey)
	p.redisClient.Expire(ctx, searchesKey, 24*time.Hour)

	processed.Metrics = append(processed.Metrics, Metric{
		Name:      "search_count",
		Value:     float64(searches),
		Unit:      "count",
		Tags:      map[string]string{},
		Timestamp: event.Timestamp,
		TTL:       24 * time.Hour,
	})

	// Track popular search terms
	if query != "" {
		termKey := fmt.Sprintf("search_term:%s", strings.ToLower(query))
		termCount, _ := p.redisClient.Incr(ctx, termKey)
		p.redisClient.Expire(ctx, termKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "search_term_count",
			Value:     float64(termCount),
			Unit:      "count",
			Tags:      map[string]string{"query": query},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	// Track search result distribution
	if resultsCount >= 0 {
		var resultsBucket string
		switch {
		case resultsCount == 0:
			resultsBucket = "0"
		case resultsCount <= 10:
			resultsBucket = "1-10"
		case resultsCount <= 50:
			resultsBucket = "11-50"
		case resultsCount <= 100:
			resultsBucket = "51-100"
		default:
			resultsBucket = "100+"
		}

		bucketKey := fmt.Sprintf("search_results_bucket:%s", resultsBucket)
		bucketCount, _ := p.redisClient.Incr(ctx, bucketKey)
		p.redisClient.Expire(ctx, bucketKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "search_results_distribution",
			Value:     float64(bucketCount),
			Unit:      "count",
			Tags:      map[string]string{"results_bucket": resultsBucket},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	return processed, nil
}

func (p *SearchEventProcessor) processSearchResultClicked(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	position, _ := event.Properties["position"].(float64)
	
	// Track click-through rates by position
	if position > 0 {
		positionKey := fmt.Sprintf("search_clicks_position:%d", int(position))
		clickCount, _ := p.redisClient.Incr(ctx, positionKey)
		p.redisClient.Expire(ctx, positionKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "search_click_by_position",
			Value:     float64(clickCount),
			Unit:      "count",
			Tags:      map[string]string{"position": fmt.Sprintf("%.0f", position)},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	return processed, nil
}

func (p *SearchEventProcessor) processSearchFilterApplied(ctx context.Context, event *AnalyticsEvent, processed *ProcessedEvent) (*ProcessedEvent, error) {
	filterType, _ := event.Properties["filter_type"].(string)
	filterValue, _ := event.Properties["filter_value"].(string)

	// Track filter usage
	if filterType != "" {
		filterKey := fmt.Sprintf("search_filter:%s", filterType)
		filterCount, _ := p.redisClient.Incr(ctx, filterKey)
		p.redisClient.Expire(ctx, filterKey, 24*time.Hour)

		processed.Metrics = append(processed.Metrics, Metric{
			Name:      "search_filter_usage",
			Value:     float64(filterCount),
			Unit:      "count",
			Tags:      map[string]string{"filter_type": filterType},
			Timestamp: event.Timestamp,
			TTL:       24 * time.Hour,
		})
	}

	return processed, nil
}