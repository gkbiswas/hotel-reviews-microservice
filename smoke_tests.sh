#!/bin/bash

# Hotel Reviews Microservice Smoke Tests
# This script tests happy and unhappy paths for the API

set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Base URL
BASE_URL="${BASE_URL:-http://localhost:8080}"
API_URL="${BASE_URL}/api/v1"

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Helper functions
print_header() {
    echo -e "\n${BLUE}===================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}===================================================${NC}"
}

print_test() {
    echo -e "\n${YELLOW}TEST: $1${NC}"
    ((TOTAL_TESTS++))
}

print_success() {
    echo -e "${GREEN}✓ PASSED: $1${NC}"
    ((PASSED_TESTS++))
}

print_failure() {
    echo -e "${RED}✗ FAILED: $1${NC}"
    echo -e "${RED}  Expected: $2${NC}"
    echo -e "${RED}  Actual: $3${NC}"
    ((FAILED_TESTS++))
}

# Function to make HTTP requests and check status
make_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    local expected_status=$4
    local description=$5
    
    print_test "$description"
    
    if [ "$method" = "GET" ] || [ "$method" = "DELETE" ]; then
        response=$(curl -s -w "\n%{http_code}" -X $method "$endpoint" -H "Content-Type: application/json")
    else
        response=$(curl -s -w "\n%{http_code}" -X $method "$endpoint" -H "Content-Type: application/json" -d "$data")
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" = "$expected_status" ]; then
        print_success "$description (Status: $http_code)"
        echo "Response: $body"
        echo "$body"
    else
        print_failure "$description" "$expected_status" "$http_code"
        echo "Response: $body"
        return 1
    fi
}

# Start smoke tests
echo -e "${BLUE}Starting Hotel Reviews Microservice Smoke Tests${NC}"
echo -e "${BLUE}Target: $BASE_URL${NC}"

# ===========================
# INFRASTRUCTURE HEALTH CHECKS
# ===========================
print_header "Infrastructure Health Checks"

print_test "Health Check Endpoint"
health_response=$(make_request "GET" "$BASE_URL/health" "" "200" "Service health check")
if [[ $health_response == *'"status":"ok"'* ]]; then
    print_success "All systems operational"
fi

print_test "Metrics Endpoint"
metrics_response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/metrics")
if [ "$metrics_response" = "200" ]; then
    print_success "Metrics endpoint accessible"
else
    print_failure "Metrics endpoint" "200" "$metrics_response"
fi

# ===========================
# HAPPY PATH TESTS
# ===========================
print_header "Happy Path Tests"

# Test 1: Get all reviews (empty initially)
make_request "GET" "$API_URL/reviews" "" "200" "Get all reviews (empty list)"

# Test 2: Create a hotel first (if endpoint exists)
print_test "Create a hotel"
hotel_data='{
    "name": "Grand Test Hotel",
    "address": "123 Test Street",
    "city": "Test City",
    "country": "Test Country",
    "star_rating": 5,
    "description": "A luxurious test hotel",
    "latitude": 40.7128,
    "longitude": -74.0060
}'

# Try to create hotel, but handle if endpoint is not implemented
hotel_response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/hotels" -H "Content-Type: application/json" -d "$hotel_data")
hotel_status=$(echo "$hotel_response" | tail -n1)
hotel_body=$(echo "$hotel_response" | sed '$d')

if [ "$hotel_status" = "201" ] || [ "$hotel_status" = "200" ]; then
    print_success "Hotel created successfully"
    hotel_id=$(echo "$hotel_body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
else
    echo -e "${YELLOW}Note: Hotel creation endpoint not implemented (Status: $hotel_status)${NC}"
    # Use a dummy hotel ID for testing
    hotel_id="550e8400-e29b-41d4-a716-446655440000"
fi

# Test 3: Create a review
print_test "Create a review"
review_data='{
    "hotel_id": "'$hotel_id'",
    "provider_id": "550e8400-e29b-41d4-a716-446655440001",
    "rating": 4.5,
    "title": "Great stay!",
    "comment": "Had a wonderful experience at this hotel. The staff was friendly and the rooms were clean.",
    "reviewer_name": "John Doe",
    "reviewer_email": "john@example.com",
    "trip_type": "business",
    "language": "en"
}'

review_response=$(make_request "POST" "$API_URL/reviews" "$review_data" "201" "Create a new review")
if [[ $review_response == *'"id":'* ]] || [[ $review_response == *'"id":"'* ]]; then
    review_id=$(echo "$review_response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "Created review ID: $review_id"
fi

# Test 4: Get all reviews (should have one now)
make_request "GET" "$API_URL/reviews" "" "200" "Get all reviews (with data)"

# Test 5: Get specific review (if ID was captured)
if [ ! -z "$review_id" ]; then
    make_request "GET" "$API_URL/reviews/$review_id" "" "200" "Get specific review"
fi

# Test 6: Update review
if [ ! -z "$review_id" ]; then
    update_data='{
        "rating": 5.0,
        "title": "Amazing stay - Updated!",
        "comment": "After thinking about it more, this deserves 5 stars!"
    }'
    make_request "PUT" "$API_URL/reviews/$review_id" "$update_data" "200" "Update existing review"
fi

# Test 7: Test pagination
make_request "GET" "$API_URL/reviews?limit=10&offset=0" "" "200" "Get reviews with pagination"

# Test 8: Test filtering/search
make_request "GET" "$API_URL/reviews?rating_min=4.0" "" "200" "Filter reviews by minimum rating"

# ===========================
# UNHAPPY PATH TESTS
# ===========================
print_header "Unhappy Path Tests"

# Test 1: Invalid JSON
make_request "POST" "$API_URL/reviews" "{invalid json}" "400" "Create review with invalid JSON"

# Test 2: Missing required fields
incomplete_review='{
    "rating": 5.0,
    "comment": "Missing hotel_id field"
}'
make_request "POST" "$API_URL/reviews" "$incomplete_review" "400" "Create review with missing fields"

# Test 3: Invalid data types
invalid_review='{
    "hotel_id": "'$hotel_id'",
    "rating": "not_a_number",
    "comment": "Invalid rating type"
}'
make_request "POST" "$API_URL/reviews" "$invalid_review" "400" "Create review with invalid data types"

# Test 4: Rating out of range
out_of_range_review='{
    "hotel_id": "'$hotel_id'",
    "rating": 10.0,
    "comment": "Rating should be between 1 and 5"
}'
make_request "POST" "$API_URL/reviews" "$out_of_range_review" "400" "Create review with rating out of range"

# Test 5: Non-existent resource
make_request "GET" "$API_URL/reviews/non-existent-id" "" "404" "Get non-existent review"

# Test 6: Update non-existent review
make_request "PUT" "$API_URL/reviews/non-existent-id" "$update_data" "404" "Update non-existent review"

# Test 7: Delete non-existent review
make_request "DELETE" "$API_URL/reviews/non-existent-id" "" "404" "Delete non-existent review"

# Test 8: Invalid query parameters
make_request "GET" "$API_URL/reviews?limit=invalid" "" "400" "Get reviews with invalid query params"

# Test 9: Exceed limits
make_request "GET" "$API_URL/reviews?limit=10000" "" "400" "Get reviews exceeding max limit"

# ===========================
# AUTHENTICATION TESTS
# ===========================
print_header "Authentication and Authorization Tests"

# Test 1: Register a new user
register_data='{
    "username": "testuser",
    "email": "test@example.com",
    "password": "SecurePassword123!",
    "first_name": "Test",
    "last_name": "User"
}'
auth_response=$(make_request "POST" "$API_URL/auth/register" "$register_data" "201" "Register new user")

# Test 2: Login with credentials
login_data='{
    "email": "test@example.com",
    "password": "SecurePassword123!"
}'
login_response=$(make_request "POST" "$API_URL/auth/login" "$login_data" "200" "Login with valid credentials")

# Extract token if login successful
if [[ $login_response == *'"token":'* ]] || [[ $login_response == *'"access_token":'* ]]; then
    token=$(echo "$login_response" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    if [ -z "$token" ]; then
        token=$(echo "$login_response" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    fi
    echo "Got auth token: ${token:0:20}..."
fi

# Test 3: Invalid login
invalid_login='{
    "email": "test@example.com",
    "password": "WrongPassword"
}'
make_request "POST" "$API_URL/auth/login" "$invalid_login" "401" "Login with invalid credentials"

# Test 4: Access protected endpoint without token
print_test "Access protected endpoint without auth"
protected_response=$(curl -s -w "\n%{http_code}" -X GET "$API_URL/protected/profile")
protected_status=$(echo "$protected_response" | tail -n1)
if [ "$protected_status" = "401" ]; then
    print_success "Protected endpoint requires authentication"
else
    print_failure "Protected endpoint should require auth" "401" "$protected_status"
fi

# Test 5: Access protected endpoint with token
if [ ! -z "$token" ]; then
    print_test "Access protected endpoint with auth"
    auth_response=$(curl -s -w "\n%{http_code}" -X GET "$API_URL/protected/profile" -H "Authorization: Bearer $token")
    auth_status=$(echo "$auth_response" | tail -n1)
    if [ "$auth_status" = "200" ]; then
        print_success "Protected endpoint accessible with valid token"
    else
        echo -e "${YELLOW}Note: Protected endpoint returned $auth_status${NC}"
    fi
fi

# ===========================
# FILE UPLOAD TESTS
# ===========================
print_header "File Upload and Processing Tests"

# Test 1: Upload a JSONL file
print_test "Upload JSONL file for processing"
# Create a test JSONL file
cat > test_reviews.jsonl << EOF
{"hotel_name":"Test Hotel 1","rating":4.5,"comment":"Great hotel!","reviewer_name":"Alice","review_date":"2024-01-01"}
{"hotel_name":"Test Hotel 2","rating":3.0,"comment":"Average experience","reviewer_name":"Bob","review_date":"2024-01-02"}
{"hotel_name":"Test Hotel 3","rating":5.0,"comment":"Excellent!","reviewer_name":"Charlie","review_date":"2024-01-03"}
EOF

# Upload file (this endpoint might not be implemented)
upload_response=$(curl -s -w "\n%{http_code}" -X POST "$API_URL/reviews/upload" -F "file=@test_reviews.jsonl" -F "provider_id=$hotel_id")
upload_status=$(echo "$upload_response" | tail -n1)
upload_body=$(echo "$upload_response" | sed '$d')

if [ "$upload_status" = "200" ] || [ "$upload_status" = "202" ]; then
    print_success "File upload initiated"
    echo "Response: $upload_body"
else
    echo -e "${YELLOW}Note: File upload endpoint not implemented or different format expected (Status: $upload_status)${NC}"
fi

# Clean up test file
rm -f test_reviews.jsonl

# ===========================
# RESILIENCE TESTS
# ===========================
print_header "Resilience and Circuit Breaker Tests"

# Test 1: Rapid requests to test rate limiting
print_test "Rate limiting test"
rate_limit_hit=false
for i in {1..20}; do
    response_code=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/reviews")
    if [ "$response_code" = "429" ]; then
        rate_limit_hit=true
        break
    fi
done

if $rate_limit_hit; then
    print_success "Rate limiting is active"
else
    echo -e "${YELLOW}Note: No rate limiting detected in 20 requests${NC}"
fi

# Test 2: Large payload test
print_test "Large payload rejection"
large_comment=$(python3 -c "print('x' * 10000)")
large_review='{
    "hotel_id": "'$hotel_id'",
    "rating": 4.0,
    "comment": "'$large_comment'"
}'
make_request "POST" "$API_URL/reviews" "$large_review" "413" "Create review with oversized payload"

# ===========================
# CACHE TESTS
# ===========================
print_header "Caching Behavior Tests"

# Test 1: Make same request multiple times and check response times
print_test "Cache performance test"
echo "Making 3 identical requests to check caching..."

for i in {1..3}; do
    start_time=$(date +%s%N)
    curl -s "$API_URL/reviews?limit=10" > /dev/null
    end_time=$(date +%s%N)
    elapsed=$((($end_time - $start_time) / 1000000))
    echo "Request $i: ${elapsed}ms"
done

# ===========================
# SUMMARY
# ===========================
print_header "Test Summary"

echo -e "${BLUE}Total Tests: $TOTAL_TESTS${NC}"
echo -e "${GREEN}Passed: $PASSED_TESTS${NC}"
echo -e "${RED}Failed: $FAILED_TESTS${NC}"
echo -e "${BLUE}Success Rate: $(( PASSED_TESTS * 100 / TOTAL_TESTS ))%${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}All smoke tests passed! 🎉${NC}"
    exit 0
else
    echo -e "\n${RED}Some tests failed. Please review the failures above.${NC}"
    exit 1
fi