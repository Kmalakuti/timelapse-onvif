#!/bin/bash
# Phase 3 Testing Script
# Run this on the testing machine with Docker

set -e

echo "=========================================="
echo "  Phase 3 Testing - TimeLapse Camera"
echo "=========================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results file
RESULTS_FILE="test_results_$(date +%Y%m%d_%H%M%S).txt"
echo "Test Results - $(date)" > $RESULTS_FILE

# Function to log results
log_result() {
    local test_name=$1
    local status=$2
    local details=$3
    echo "[$status] $test_name: $details" >> $RESULTS_FILE
    if [ "$status" == "PASS" ]; then
        echo -e "${GREEN}[PASS]${NC} $test_name"
    else
        echo -e "${RED}[FAIL]${NC} $test_name: $details"
    fi
}

echo "Step 1: Building Docker image..."
docker-compose build

echo ""
echo "Step 2: Running Unit Tests..."
echo "=========================================="
if docker-compose run --rm timelapse-dev go test ./internal/... -v 2>&1 | tee unit_tests.log; then
    log_result "Unit Tests" "PASS" "All tests passed"
else
    log_result "Unit Tests" "FAIL" "See unit_tests.log"
fi

echo ""
echo "Step 3: Starting API Server..."
echo "=========================================="
docker-compose up -d timelapse-dev
sleep 10  # Wait for server to start

echo ""
echo "Step 4: Testing API Endpoints..."
echo "=========================================="

# Test 1: Health check
echo "Test 1: Health Check"
HEALTH=$(curl -s http://localhost:8000/health)
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    log_result "Health Check" "PASS" "$HEALTH"
else
    log_result "Health Check" "FAIL" "$HEALTH"
fi

# Test 2: List cameras
echo "Test 2: List Cameras"
CAMERAS=$(curl -s http://localhost:8000/api/v1/cameras)
if echo "$CAMERAS" | grep -q '"uuid"'; then
    log_result "List Cameras" "PASS" "Cameras returned"
else
    log_result "List Cameras" "FAIL" "$CAMERAS"
fi

# Test 3: Get statistics
echo "Test 3: Get Statistics"
STATS=$(curl -s http://localhost:8000/api/v1/stats)
if echo "$STATS" | grep -q '"cameras"'; then
    log_result "Get Statistics" "PASS" "Stats returned"
else
    log_result "Get Statistics" "FAIL" "$STATS"
fi

# Test 4: Probe camera (update credentials as needed)
echo "Test 4: Probe Camera"
PROBE=$(curl -s -X POST http://localhost:8000/api/v1/discovery/probe \
  -H "Content-Type: application/json" \
  -d "{\"ip\":\"192.168.200.13\",\"port\":80,\"username\":\"admin\",\"password\":\"${TIMELAPSE_TEST_CAMERA_PASSWORD:-YOUR_CAMERA_PASSWORD}\"}")
if echo "$PROBE" | grep -q '"success":true'; then
    log_result "Probe Camera" "PASS" "Camera found"
else
    log_result "Probe Camera" "FAIL" "$PROBE"
fi

# Test 5: Get camera profiles (need to extract UUID first)
echo "Test 5: List Profiles"
UUID=$(echo $CAMERAS | grep -o '"uuid":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$UUID" ]; then
    PROFILES=$(curl -s "http://localhost:8000/api/v1/cameras/$UUID/profiles")
    if echo "$PROFILES" | grep -q '"profiles"'; then
        log_result "List Profiles" "PASS" "Profiles returned"
    else
        log_result "List Profiles" "FAIL" "$PROFILES"
    fi
else
    log_result "List Profiles" "SKIP" "No camera UUID available"
fi

# Test 6: Take snapshot
echo "Test 6: Take Snapshot"
if [ -n "$UUID" ]; then
    SNAPSHOT=$(curl -s -X POST "http://localhost:8000/api/v1/cameras/$UUID/snapshot")
    if echo "$SNAPSHOT" | grep -q '"success":true'; then
        log_result "Take Snapshot" "PASS" "Snapshot captured"
    else
        log_result "Take Snapshot" "FAIL" "$SNAPSHOT"
    fi
else
    log_result "Take Snapshot" "SKIP" "No camera UUID available"
fi

# Test 7: List images
echo "Test 7: List Images"
if [ -n "$UUID" ]; then
    IMAGES=$(curl -s "http://localhost:8000/api/v1/cameras/$UUID/images")
    if echo "$IMAGES" | grep -q '"images"'; then
        log_result "List Images" "PASS" "Images returned"
    else
        log_result "List Images" "FAIL" "$IMAGES"
    fi
else
    log_result "List Images" "SKIP" "No camera UUID available"
fi

echo ""
echo "=========================================="
echo "  Test Summary"
echo "=========================================="
cat $RESULTS_FILE

echo ""
echo "Step 5: Cleanup"
docker-compose down

echo ""
echo "Results saved to: $RESULTS_FILE"
echo "Unit test log saved to: unit_tests.log"
echo ""
echo "Done! Copy results to PHASE3_COMPLETE.md"
