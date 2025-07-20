#!/bin/bash

# Quick smoke tests for Hotel Reviews Microservice
set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BASE_URL="http://localhost:8080"
API_URL="${BASE_URL}/api/v1"

echo -e "${BLUE}=== Quick Smoke Tests ===${NC}\n"

# Test 1: Health Check
echo -e "${YELLOW}1. Testing Health Check${NC}"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "${BASE_URL}/health")
status=$(echo "$response" | grep "HTTP_STATUS" | cut -d: -f2)
body=$(echo "$response" | grep -v "HTTP_STATUS")

if [ "$status" = "200" ]; then
    echo -e "${GREEN}✓ Health check passed${NC}"
    echo "Response: $body"
else
    echo -e "${RED}✗ Health check failed (Status: $status)${NC}"
fi

# Test 2: Metrics
echo -e "\n${YELLOW}2. Testing Metrics Endpoint${NC}"
metrics_status=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/metrics")
if [ "$metrics_status" = "200" ]; then
    echo -e "${GREEN}✓ Metrics endpoint accessible${NC}"
else
    echo -e "${RED}✗ Metrics endpoint failed (Status: $metrics_status)${NC}"
fi

# Test 3: Get Reviews (Happy Path)
echo -e "\n${YELLOW}3. Testing Get Reviews (Happy Path)${NC}"
reviews_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "${API_URL}/reviews")
reviews_status=$(echo "$reviews_response" | grep "HTTP_STATUS" | cut -d: -f2)
reviews_body=$(echo "$reviews_response" | grep -v "HTTP_STATUS")

if [ "$reviews_status" = "200" ]; then
    echo -e "${GREEN}✓ Get reviews successful${NC}"
    echo "Response: $reviews_body"
else
    echo -e "${RED}✗ Get reviews failed (Status: $reviews_status)${NC}"
    echo "Response: $reviews_body"
fi

# Test 4: Create Review (Happy Path)
echo -e "\n${YELLOW}4. Testing Create Review (Happy Path)${NC}"
review_data='{
    "hotel_id": "550e8400-e29b-41d4-a716-446655440000",
    "provider_id": "550e8400-e29b-41d4-a716-446655440001",
    "rating": 4.5,
    "title": "Great hotel!",
    "comment": "Had a wonderful stay. The staff was friendly and helpful.",
    "reviewer_name": "John Doe",
    "reviewer_email": "john@example.com"
}'

create_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "${API_URL}/reviews" \
    -H "Content-Type: application/json" \
    -d "$review_data")
create_status=$(echo "$create_response" | grep "HTTP_STATUS" | cut -d: -f2)
create_body=$(echo "$create_response" | grep -v "HTTP_STATUS")

if [ "$create_status" = "201" ] || [ "$create_status" = "200" ]; then
    echo -e "${GREEN}✓ Create review successful${NC}"
    echo "Response: $create_body"
    
    # Extract review ID if possible
    review_id=$(echo "$create_body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    if [ ! -z "$review_id" ]; then
        echo "Created review ID: $review_id"
    fi
else
    echo -e "${RED}✗ Create review failed (Status: $create_status)${NC}"
    echo "Response: $create_body"
fi

# Test 5: Invalid Request (Unhappy Path)
echo -e "\n${YELLOW}5. Testing Invalid Request (Unhappy Path)${NC}"
invalid_data='{"invalid": "data", "rating": "not_a_number"}'
invalid_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "${API_URL}/reviews" \
    -H "Content-Type: application/json" \
    -d "$invalid_data")
invalid_status=$(echo "$invalid_response" | grep "HTTP_STATUS" | cut -d: -f2)
invalid_body=$(echo "$invalid_response" | grep -v "HTTP_STATUS")

if [ "$invalid_status" = "400" ]; then
    echo -e "${GREEN}✓ Invalid request properly rejected${NC}"
    echo "Response: $invalid_body"
else
    echo -e "${YELLOW}⚠ Invalid request handling (Status: $invalid_status)${NC}"
    echo "Response: $invalid_body"
fi

# Test 6: Non-existent Resource (Unhappy Path)
echo -e "\n${YELLOW}6. Testing Non-existent Resource (Unhappy Path)${NC}"
notfound_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "${API_URL}/reviews/non-existent-id")
notfound_status=$(echo "$notfound_response" | grep "HTTP_STATUS" | cut -d: -f2)
notfound_body=$(echo "$notfound_response" | grep -v "HTTP_STATUS")

if [ "$notfound_status" = "404" ]; then
    echo -e "${GREEN}✓ Non-existent resource properly handled${NC}"
    echo "Response: $notfound_body"
else
    echo -e "${YELLOW}⚠ Non-existent resource handling (Status: $notfound_status)${NC}"
    echo "Response: $notfound_body"
fi

# Test 7: Authentication Test
echo -e "\n${YELLOW}7. Testing Authentication${NC}"
auth_response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" "${API_URL}/protected/profile")
auth_status=$(echo "$auth_response" | grep "HTTP_STATUS" | cut -d: -f2)
auth_body=$(echo "$auth_response" | grep -v "HTTP_STATUS")

if [ "$auth_status" = "401" ] || [ "$auth_status" = "403" ]; then
    echo -e "${GREEN}✓ Authentication protection working${NC}"
    echo "Response: $auth_body"
else
    echo -e "${YELLOW}⚠ Authentication behavior (Status: $auth_status)${NC}"
    echo "Response: $auth_body"
fi

# Test 8: Performance Test
echo -e "\n${YELLOW}8. Testing Response Performance${NC}"
echo "Making 5 requests to measure performance..."

total_time=0
for i in {1..5}; do
    start_time=$(date +%s%N)
    curl -s "${API_URL}/reviews" > /dev/null
    end_time=$(date +%s%N)
    elapsed=$((($end_time - $start_time) / 1000000))
    echo "Request $i: ${elapsed}ms"
    total_time=$((total_time + elapsed))
done

avg_time=$((total_time / 5))
echo -e "${GREEN}✓ Average response time: ${avg_time}ms${NC}"

if [ $avg_time -lt 1000 ]; then
    echo -e "${GREEN}✓ Performance is excellent (< 1 second)${NC}"
elif [ $avg_time -lt 5000 ]; then
    echo -e "${YELLOW}⚠ Performance is acceptable (< 5 seconds)${NC}"
else
    echo -e "${RED}✗ Performance needs improvement (> 5 seconds)${NC}"
fi

echo -e "\n${BLUE}=== Quick Tests Complete ===${NC}"