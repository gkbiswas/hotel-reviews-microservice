package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Provider represents a review provider (e.g., Booking.com, Expedia)
type Provider struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()" validate:"required"`
	Name        string         `json:"name" gorm:"type:varchar(100);not null;unique" validate:"required,min=1,max=100"`
	BaseURL     string         `json:"base_url" gorm:"type:varchar(255)" validate:"url"`
	IsActive    bool           `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// Hotel represents a hotel entity
type Hotel struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()" validate:"required"`
	Name        string         `json:"name" gorm:"type:varchar(255);not null" validate:"required,min=1,max=255"`
	Address     string         `json:"address" gorm:"type:text"`
	City        string         `json:"city" gorm:"type:varchar(100)" validate:"max=100"`
	Country     string         `json:"country" gorm:"type:varchar(100)" validate:"max=100"`
	PostalCode  string         `json:"postal_code" gorm:"type:varchar(20)" validate:"max=20"`
	Phone       string         `json:"phone" gorm:"type:varchar(20)" validate:"max=20"`
	Email       string         `json:"email" gorm:"type:varchar(255)" validate:"email,max=255"`
	StarRating  int            `json:"star_rating" gorm:"type:int;check:star_rating >= 1 AND star_rating <= 5" validate:"min=1,max=5"`
	Description string         `json:"description" gorm:"type:text"`
	Amenities   []string       `json:"amenities" gorm:"type:jsonb"`
	Latitude    float64        `json:"latitude" gorm:"type:decimal(10,8)" validate:"min=-90,max=90"`
	Longitude   float64        `json:"longitude" gorm:"type:decimal(11,8)" validate:"min=-180,max=180"`
	CreatedAt   time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
	
	// Relationships
	Reviews []Review `json:"reviews,omitempty" gorm:"foreignKey:HotelID"`
}

// ReviewerInfo represents reviewer information
type ReviewerInfo struct {
	ID               uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()" validate:"required"`
	Name             string         `json:"name" gorm:"type:varchar(255)" validate:"max=255"`
	Email            string         `json:"email" gorm:"type:varchar(255)" validate:"email,max=255"`
	Country          string         `json:"country" gorm:"type:varchar(100)" validate:"max=100"`
	IsVerified       bool           `json:"is_verified" gorm:"default:false"`
	TotalReviews     int            `json:"total_reviews" gorm:"default:0"`
	AverageRating    float64        `json:"average_rating" gorm:"type:decimal(3,2);default:0.0"`
	MemberSince      *time.Time     `json:"member_since,omitempty"`
	ProfileImageURL  string         `json:"profile_image_url" gorm:"type:varchar(500)" validate:"url,max=500"`
	Bio              string         `json:"bio" gorm:"type:text"`
	CreatedAt        time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt        time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt        gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
	
	// Relationships
	Reviews []Review `json:"reviews,omitempty" gorm:"foreignKey:ReviewerInfoID"`
}

// Review represents a hotel review
type Review struct {
	ID             uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()" validate:"required"`
	ProviderID     uuid.UUID      `json:"provider_id" gorm:"type:uuid;not null" validate:"required"`
	HotelID        uuid.UUID      `json:"hotel_id" gorm:"type:uuid;not null" validate:"required"`
	ReviewerInfoID uuid.UUID      `json:"reviewer_info_id" gorm:"type:uuid;not null" validate:"required"`
	ExternalID     string         `json:"external_id" gorm:"type:varchar(255)" validate:"max=255"`
	Rating         float64        `json:"rating" gorm:"type:decimal(3,2);not null;check:rating >= 1.0 AND rating <= 5.0" validate:"required,min=1.0,max=5.0"`
	Title          string         `json:"title" gorm:"type:varchar(500)" validate:"max=500"`
	Comment        string         `json:"comment" gorm:"type:text"`
	ReviewDate     time.Time      `json:"review_date" gorm:"not null" validate:"required"`
	StayDate       *time.Time     `json:"stay_date,omitempty"`
	TripType       string         `json:"trip_type" gorm:"type:varchar(50)" validate:"max=50"`
	RoomType       string         `json:"room_type" gorm:"type:varchar(100)" validate:"max=100"`
	IsVerified     bool           `json:"is_verified" gorm:"default:false"`
	HelpfulVotes   int            `json:"helpful_votes" gorm:"default:0"`
	TotalVotes     int            `json:"total_votes" gorm:"default:0"`
	Language       string         `json:"language" gorm:"type:varchar(10);default:'en'" validate:"max=10"`
	Sentiment      string         `json:"sentiment" gorm:"type:varchar(20)" validate:"max=20"`
	Source         string         `json:"source" gorm:"type:varchar(100)" validate:"max=100"`
	
	// Detailed ratings
	ServiceRating     *float64 `json:"service_rating,omitempty" gorm:"type:decimal(3,2);check:service_rating >= 1.0 AND service_rating <= 5.0" validate:"omitempty,min=1.0,max=5.0"`
	CleanlinessRating *float64 `json:"cleanliness_rating,omitempty" gorm:"type:decimal(3,2);check:cleanliness_rating >= 1.0 AND cleanliness_rating <= 5.0" validate:"omitempty,min=1.0,max=5.0"`
	LocationRating    *float64 `json:"location_rating,omitempty" gorm:"type:decimal(3,2);check:location_rating >= 1.0 AND location_rating <= 5.0" validate:"omitempty,min=1.0,max=5.0"`
	ValueRating       *float64 `json:"value_rating,omitempty" gorm:"type:decimal(3,2);check:value_rating >= 1.0 AND value_rating <= 5.0" validate:"omitempty,min=1.0,max=5.0"`
	ComfortRating     *float64 `json:"comfort_rating,omitempty" gorm:"type:decimal(3,2);check:comfort_rating >= 1.0 AND comfort_rating <= 5.0" validate:"omitempty,min=1.0,max=5.0"`
	FacilitiesRating  *float64 `json:"facilities_rating,omitempty" gorm:"type:decimal(3,2);check:facilities_rating >= 1.0 AND facilities_rating <= 5.0" validate:"omitempty,min=1.0,max=5.0"`
	
	// Metadata
	Metadata       map[string]interface{} `json:"metadata,omitempty" gorm:"type:jsonb"`
	ProcessedAt    *time.Time             `json:"processed_at,omitempty"`
	ProcessingHash string                 `json:"processing_hash" gorm:"type:varchar(64)" validate:"max=64"`
	
	// Timestamps
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
	
	// Relationships
	Provider     Provider     `json:"provider" gorm:"foreignKey:ProviderID"`
	Hotel        Hotel        `json:"hotel" gorm:"foreignKey:HotelID"`
	ReviewerInfo ReviewerInfo `json:"reviewer_info" gorm:"foreignKey:ReviewerInfoID"`
}

// ReviewSummary represents aggregated review statistics for a hotel
type ReviewSummary struct {
	ID               uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()" validate:"required"`
	HotelID          uuid.UUID      `json:"hotel_id" gorm:"type:uuid;not null;unique" validate:"required"`
	TotalReviews     int            `json:"total_reviews" gorm:"default:0"`
	AverageRating    float64        `json:"average_rating" gorm:"type:decimal(3,2);default:0.0"`
	RatingDistribution map[string]int `json:"rating_distribution" gorm:"type:jsonb"`
	
	// Average detailed ratings
	AvgServiceRating     float64 `json:"avg_service_rating" gorm:"type:decimal(3,2);default:0.0"`
	AvgCleanlinessRating float64 `json:"avg_cleanliness_rating" gorm:"type:decimal(3,2);default:0.0"`
	AvgLocationRating    float64 `json:"avg_location_rating" gorm:"type:decimal(3,2);default:0.0"`
	AvgValueRating       float64 `json:"avg_value_rating" gorm:"type:decimal(3,2);default:0.0"`
	AvgComfortRating     float64 `json:"avg_comfort_rating" gorm:"type:decimal(3,2);default:0.0"`
	AvgFacilitiesRating  float64 `json:"avg_facilities_rating" gorm:"type:decimal(3,2);default:0.0"`
	
	LastReviewDate time.Time      `json:"last_review_date"`
	CreatedAt      time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt      gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
	
	// Relationships
	Hotel Hotel `json:"hotel" gorm:"foreignKey:HotelID"`
}

// ReviewProcessingStatus represents the status of review processing
type ReviewProcessingStatus struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()" validate:"required"`
	ProviderID  uuid.UUID      `json:"provider_id" gorm:"type:uuid;not null" validate:"required"`
	Status      string         `json:"status" gorm:"type:varchar(50);not null" validate:"required,oneof=pending processing completed failed"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	ErrorMsg    string         `json:"error_msg,omitempty" gorm:"type:text"`
	RecordsProcessed int        `json:"records_processed" gorm:"default:0"`
	RecordsTotal     int        `json:"records_total" gorm:"default:0"`
	FileURL     string         `json:"file_url" gorm:"type:varchar(500)" validate:"url,max=500"`
	CreatedAt   time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
	
	// Relationships
	Provider Provider `json:"provider" gorm:"foreignKey:ProviderID"`
}