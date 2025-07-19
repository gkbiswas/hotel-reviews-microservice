package domain

import (
	"github.com/google/uuid"
)

// ReviewFilter represents filters for review queries
type ReviewFilter struct {
	HotelID    *uuid.UUID `json:"hotel_id,omitempty"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	ProviderID *uuid.UUID `json:"provider_id,omitempty"`
	MinRating  float64    `json:"min_rating,omitempty"`
	MaxRating  float64    `json:"max_rating,omitempty"`
	Language   string     `json:"language,omitempty"`
	TripType   string     `json:"trip_type,omitempty"`
	IsVerified *bool      `json:"is_verified,omitempty"`
	DateFrom   string     `json:"date_from,omitempty"`
	DateTo     string     `json:"date_to,omitempty"`
	Limit      int        `json:"limit,omitempty"`
	Offset     int        `json:"offset,omitempty"`
	SortBy     string     `json:"sort_by,omitempty"`
	SortDesc   bool       `json:"sort_desc,omitempty"`
}

// HotelFilter represents filters for hotel queries
type HotelFilter struct {
	City          string   `json:"city,omitempty"`
	Country       string   `json:"country,omitempty"`
	MinStarRating int      `json:"min_star_rating,omitempty"`
	MaxStarRating int      `json:"max_star_rating,omitempty"`
	SearchTerm    string   `json:"search_term,omitempty"`
	Latitude      float64  `json:"latitude,omitempty"`
	Longitude     float64  `json:"longitude,omitempty"`
	RadiusKm      float64  `json:"radius_km,omitempty"`
	Amenities     []string `json:"amenities,omitempty"`
	Limit         int      `json:"limit,omitempty"`
	Offset        int      `json:"offset,omitempty"`
	SortBy        string   `json:"sort_by,omitempty"`
	SortDesc      bool     `json:"sort_desc,omitempty"`
}

// UserFilter represents filters for user queries
type UserFilter struct {
	Role       string `json:"role,omitempty"`
	IsActive   *bool  `json:"is_active,omitempty"`
	Verified   *bool  `json:"verified,omitempty"`
	SearchTerm string `json:"search_term,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	SortDesc   bool   `json:"sort_desc,omitempty"`
}
