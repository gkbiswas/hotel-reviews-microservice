#!/bin/bash

# Hotel Reviews API - cURL Examples
# This script demonstrates how to use the Hotel Reviews API with cURL commands

set -e

# Configuration
API_BASE="${API_BASE:-https://api.hotelreviews.com/api/v1}"
STAGING_BASE="https://staging-api.hotelreviews.com/api/v1"
ACCESS_TOKEN=""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Helper function to make authenticated requests
auth_curl() {
    if [ -z "$ACCESS_TOKEN" ]; then
        echo -e "${RED}Error: ACCESS_TOKEN not set. Please authenticate first.${NC}"
        return 1
    fi
    
    curl -H "Authorization: Bearer $ACCESS_TOKEN" \
         -H "Content-Type: application/json" \
         "$@"
}

# Helper function to pretty print JSON
pretty_json() {
    if command -v jq &> /dev/null; then
        jq .
    else
        cat
    fi
}

echo -e "${BLUE}Hotel Reviews API - cURL Examples${NC}"
echo -e "${BLUE}=================================${NC}"
echo ""
echo "API Base URL: $API_BASE"
echo "Staging URL: $STAGING_BASE"
echo ""

# Example 1: Health Check
echo -e "${GREEN}Example 1: Health Check${NC}"
echo "This endpoint doesn't require authentication"
echo ""
echo "Command:"
echo "curl -X GET $API_BASE/health"
echo ""
echo "Response:"
curl -X GET "$API_BASE/health" 2>/dev/null | pretty_json
echo ""
echo "---"
echo ""

# Example 2: User Registration
echo -e "${GREEN}Example 2: User Registration${NC}"
echo "Register a new user account"
echo ""
echo "Command:"
cat << 'EOF'
curl -X POST $API_BASE/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser_$(date +%s)",
    "email": "testuser_$(date +%s)@example.com",
    "password": "SecurePassword123!",
    "first_name": "Test",
    "last_name": "User"
  }'
EOF
echo ""
echo "Example Response:"
cat << 'EOF'
{
  "success": true,
  "data": {
    "user": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "username": "testuser_1642249200",
      "email": "testuser_1642249200@example.com",
      "first_name": "Test",
      "last_name": "User",
      "is_verified": false
    },
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
EOF
echo ""
echo "---"
echo ""

# Example 3: User Login
echo -e "${GREEN}Example 3: User Login${NC}"
echo "Authenticate and get access token"
echo ""
echo "Command:"
cat << 'EOF'
curl -X POST $API_BASE/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john.doe@example.com",
    "password": "SecurePassword123!"
  }'
EOF
echo ""
echo "Save the access_token from the response for subsequent requests."
echo ""
echo "---"
echo ""

# Check if user wants to continue with authenticated examples
echo -e "${YELLOW}For the following examples, you need an access token.${NC}"
echo "You can:"
echo "1. Set ACCESS_TOKEN environment variable: export ACCESS_TOKEN=\"your_token_here\""
echo "2. Use staging environment for testing"
echo ""

if [ -z "$ACCESS_TOKEN" ]; then
    echo -e "${YELLOW}ACCESS_TOKEN not set. Showing command examples without executing them.${NC}"
    echo ""
fi

# Example 4: List Reviews
echo -e "${GREEN}Example 4: List Reviews${NC}"
echo "Get a paginated list of reviews"
echo ""
echo "Basic listing:"
echo "curl -X GET '$API_BASE/reviews?limit=20&offset=0' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""
echo "With filtering:"
echo "curl -X GET '$API_BASE/reviews?hotel_id=123e4567-e89b-12d3-a456-426614174000&rating_min=4&limit=10' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""

if [ -n "$ACCESS_TOKEN" ]; then
    echo "Executing request..."
    auth_curl -X GET "$API_BASE/reviews?limit=5" 2>/dev/null | pretty_json
fi

echo ""
echo "---"
echo ""

# Example 5: Create a Review
echo -e "${GREEN}Example 5: Create a Review${NC}"
echo "Create a new hotel review"
echo ""
echo "Command:"
cat << 'EOF'
curl -X POST $API_BASE/reviews \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "hotel_id": "123e4567-e89b-12d3-a456-426614174000",
    "reviewer_info_id": "456e7890-e89b-12d3-a456-426614174001",
    "rating": 4.5,
    "title": "Excellent stay with great service",
    "comment": "Had a wonderful time at this hotel. The staff was incredibly friendly and the room was spotless. The location is perfect for exploring the city. Highly recommend!",
    "review_date": "2024-01-15T00:00:00Z",
    "language": "en",
    "service_rating": 5.0,
    "cleanliness_rating": 4.5,
    "location_rating": 5.0,
    "value_rating": 4.0,
    "trip_type": "leisure",
    "room_type": "deluxe"
  }'
EOF
echo ""
echo "---"
echo ""

# Example 6: Search Reviews
echo -e "${GREEN}Example 6: Search Reviews${NC}"
echo "Perform full-text search across reviews"
echo ""
echo "Basic search:"
echo "curl -X GET '$API_BASE/reviews/search?query=excellent%20service&limit=10' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""
echo "Advanced search with filters:"
echo "curl -X GET '$API_BASE/reviews/search?query=location&rating_min=4&language=en&verified_only=true' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""

if [ -n "$ACCESS_TOKEN" ]; then
    echo "Executing search request..."
    auth_curl -X GET "$API_BASE/reviews/search?query=hotel&limit=3" 2>/dev/null | pretty_json
fi

echo ""
echo "---"
echo ""

# Example 7: Get Review Statistics
echo -e "${GREEN}Example 7: Get Review Statistics${NC}"
echo "Retrieve aggregated review statistics"
echo ""
echo "All reviews statistics:"
echo "curl -X GET '$API_BASE/reviews/statistics' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""
echo "Hotel-specific statistics:"
echo "curl -X GET '$API_BASE/reviews/statistics?hotel_id=123e4567-e89b-12d3-a456-426614174000' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""
echo "Date range statistics:"
echo "curl -X GET '$API_BASE/reviews/statistics?from_date=2024-01-01&to_date=2024-01-31' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""
echo "---"
echo ""

# Example 8: List Hotels
echo -e "${GREEN}Example 8: List Hotels${NC}"
echo "Get a list of hotels with filtering"
echo ""
echo "Basic listing:"
echo "curl -X GET '$API_BASE/hotels?limit=20' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""
echo "Filter by location and rating:"
echo "curl -X GET '$API_BASE/hotels?city=London&star_rating=5&min_rating=4.0' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""

if [ -n "$ACCESS_TOKEN" ]; then
    echo "Executing request..."
    auth_curl -X GET "$API_BASE/hotels?limit=3" 2>/dev/null | pretty_json
fi

echo ""
echo "---"
echo ""

# Example 9: Create a Hotel (Admin Only)
echo -e "${GREEN}Example 9: Create a Hotel (Admin Only)${NC}"
echo "Create a new hotel entry"
echo ""
echo "Command:"
cat << 'EOF'
curl -X POST $API_BASE/hotels \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Grand Hotel Example",
    "address": "123 Main Street",
    "city": "London",
    "country": "United Kingdom",
    "postal_code": "SW1A 1AA",
    "phone": "+44 20 7123 4567",
    "email": "info@grandhotelexample.com",
    "star_rating": 5,
    "description": "A luxurious hotel in the heart of London with exceptional service and amenities.",
    "amenities": ["wifi", "restaurant", "gym", "spa", "pool", "parking"],
    "latitude": 51.5074,
    "longitude": -0.1278
  }'
EOF
echo ""
echo "---"
echo ""

# Example 10: Upload Reviews File
echo -e "${GREEN}Example 10: Upload Reviews File${NC}"
echo "Upload a CSV file for batch review processing"
echo ""
echo "Create a sample CSV file first:"
cat << 'EOF'
cat > sample_reviews.csv << 'EOL'
hotel_id,reviewer_name,rating,title,comment,review_date,language
123e4567-e89b-12d3-a456-426614174000,John Doe,4.5,"Great stay","Had a wonderful time at this hotel. Staff was friendly and rooms were clean.",2024-01-15,en
123e4567-e89b-12d3-a456-426614174000,Jane Smith,3.5,"Average experience","Hotel was okay, nothing special but met basic expectations.",2024-01-16,en
456e7890-e89b-12d3-a456-426614174001,Bob Johnson,5.0,"Excellent!","Outstanding service and facilities. Highly recommended!",2024-01-17,en
EOL
EOF
echo ""
echo "Upload command:"
cat << 'EOF'
curl -X POST $API_BASE/reviews/upload \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -F "file=@sample_reviews.csv" \
  -F "provider_id=123e4567-e89b-12d3-a456-426614174000"
EOF
echo ""
echo "Check processing status:"
echo "curl -X GET '$API_BASE/reviews/processing/{processing_id}' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""
echo "---"
echo ""

# Example 11: Bulk Create Reviews
echo -e "${GREEN}Example 11: Bulk Create Reviews${NC}"
echo "Create multiple reviews in a single request"
echo ""
echo "Command:"
cat << 'EOF'
curl -X POST $API_BASE/reviews/bulk \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "reviews": [
      {
        "hotel_id": "123e4567-e89b-12d3-a456-426614174000",
        "reviewer_info_id": "456e7890-e89b-12d3-a456-426614174001",
        "rating": 4.0,
        "title": "Good value for money",
        "comment": "Clean rooms and friendly staff.",
        "review_date": "2024-01-15T00:00:00Z",
        "language": "en"
      },
      {
        "hotel_id": "123e4567-e89b-12d3-a456-426614174000",
        "reviewer_info_id": "789e0123-e89b-12d3-a456-426614174002",
        "rating": 5.0,
        "title": "Outstanding experience",
        "comment": "Everything was perfect from check-in to check-out.",
        "review_date": "2024-01-16T00:00:00Z",
        "language": "en"
      }
    ]
  }'
EOF
echo ""
echo "---"
echo ""

# Example 12: Update a Review
echo -e "${GREEN}Example 12: Update a Review${NC}"
echo "Update an existing review (only by owner or admin)"
echo ""
echo "Command:"
cat << 'EOF'
curl -X PUT $API_BASE/reviews/{review_id} \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "rating": 5.0,
    "title": "Updated: Absolutely fantastic!",
    "comment": "After my initial review, I wanted to update it. This hotel exceeded all expectations. The service was impeccable and I will definitely return.",
    "service_rating": 5.0,
    "cleanliness_rating": 5.0
  }'
EOF
echo ""
echo "---"
echo ""

# Example 13: Get Application Metrics
echo -e "${GREEN}Example 13: Get Application Metrics (Admin)${NC}"
echo "Retrieve application performance metrics"
echo ""
echo "Command:"
echo "curl -X GET '$API_BASE/metrics' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo ""

if [ -n "$ACCESS_TOKEN" ]; then
    echo "Executing request..."
    auth_curl -X GET "$API_BASE/metrics" 2>/dev/null | pretty_json
fi

echo ""
echo "---"
echo ""

# Example 14: Error Handling Examples
echo -e "${GREEN}Example 14: Error Handling Examples${NC}"
echo "Examples of common error responses"
echo ""

echo "1. Unauthorized request (missing token):"
echo "curl -X GET '$API_BASE/reviews'"
echo ""
echo "Response:"
cat << 'EOF'
{
  "success": false,
  "error": "Authentication required",
  "error_code": "UNAUTHORIZED",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
EOF
echo ""

echo "2. Validation error:"
echo "curl -X POST '$API_BASE/reviews' \\"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN' \\"
echo "  -H 'Content-Type: application/json' \\"
echo "  -d '{\"rating\": 6}'"  # Invalid rating
echo ""
echo "Response:"
cat << 'EOF'
{
  "success": false,
  "error": "Request validation failed",
  "error_code": "VALIDATION_ERROR",
  "details": {
    "validation_errors": [
      {
        "field": "rating",
        "message": "Rating must be between 1 and 5",
        "code": "OUT_OF_RANGE"
      }
    ]
  },
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
EOF
echo ""

echo "3. Rate limit exceeded:"
cat << 'EOF'
{
  "success": false,
  "error": "Rate limit exceeded",
  "error_code": "RATE_LIMIT_EXCEEDED",
  "timestamp": "2024-01-15T10:30:00Z",
  "trace_id": "abc123def456"
}
EOF
echo ""
echo "---"
echo ""

# Example 15: Refresh Token
echo -e "${GREEN}Example 15: Refresh Access Token${NC}"
echo "Refresh an expired access token using a refresh token"
echo ""
echo "Command:"
cat << 'EOF'
curl -X POST $API_BASE/auth/refresh \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "your_refresh_token_here"
  }'
EOF
echo ""
echo "---"
echo ""

# Example 16: Circuit Breaker Health
echo -e "${GREEN}Example 16: Circuit Breaker Health Check${NC}"
echo "Check the status of circuit breakers"
echo ""
echo "Command:"
echo "curl -X GET '$API_BASE/health/circuit-breakers'"
echo ""
echo "Response:"
curl -X GET "$API_BASE/health/circuit-breakers" 2>/dev/null | pretty_json
echo ""
echo "---"
echo ""

# Testing script
echo -e "${GREEN}Example 17: Test Script${NC}"
echo "A simple script to test API functionality"
echo ""
cat << 'EOF'
#!/bin/bash
# Simple API test script

API_BASE="https://staging-api.hotelreviews.com/api/v1"
EMAIL="test@example.com"
PASSWORD="TestPassword123!"

echo "1. Health check..."
curl -f "$API_BASE/health" > /dev/null || { echo "Health check failed"; exit 1; }

echo "2. Login..."
RESPONSE=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")

TOKEN=$(echo "$RESPONSE" | jq -r '.data.access_token')
if [ "$TOKEN" = "null" ]; then
  echo "Login failed"
  exit 1
fi

echo "3. List reviews..."
curl -f -H "Authorization: Bearer $TOKEN" "$API_BASE/reviews?limit=1" > /dev/null || {
  echo "List reviews failed"
  exit 1
}

echo "4. Search reviews..."
curl -f -H "Authorization: Bearer $TOKEN" "$API_BASE/reviews/search?query=test&limit=1" > /dev/null || {
  echo "Search failed"
  exit 1
}

echo "All tests passed!"
EOF
echo ""

echo -e "${BLUE}=================================${NC}"
echo -e "${BLUE}End of cURL Examples${NC}"
echo ""
echo -e "${YELLOW}Tips:${NC}"
echo "1. Replace {review_id}, {hotel_id}, etc. with actual UUIDs"
echo "2. Set ACCESS_TOKEN environment variable for authenticated requests"
echo "3. Use jq for pretty JSON formatting: curl ... | jq ."
echo "4. Use staging environment for testing: $STAGING_BASE"
echo "5. Check response headers for rate limiting information"
echo ""
echo -e "${YELLOW}Common Headers to Include:${NC}"
echo "  -H 'Authorization: Bearer \$ACCESS_TOKEN'"
echo "  -H 'Content-Type: application/json'"
echo "  -H 'Accept: application/json'"
echo ""
echo -e "${YELLOW}Debugging:${NC}"
echo "  Add -v flag for verbose output: curl -v ..."
echo "  Add -i flag to include response headers: curl -i ..."
echo "  Use --trace-ascii /dev/stdout for detailed tracing"