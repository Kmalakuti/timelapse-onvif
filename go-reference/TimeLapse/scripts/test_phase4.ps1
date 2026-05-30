# scripts/test_phase4.ps1
# Phase 4 Feature Tests - Profile Selection, Multi-Resolution, IP Change Detection

param(
    [string]$BaseUrl = "http://127.0.0.1:8000"
)

$passed = 0
$failed = 0

function Test-Endpoint {
    param(
        [string]$Name,
        [string]$Method,
        [string]$Url,
        [string]$Body = $null
    )
    try {
        $params = @{
            Uri = $Url
            Method = $Method
            ErrorAction = "Stop"
        }
        if ($Body) {
            $params.Body = $Body
            $params.ContentType = "application/json"
        }
        $response = Invoke-RestMethod @params
        Write-Host "[PASS] $Name" -ForegroundColor Green
        $script:passed++
        return $response
    } catch {
        Write-Host "[FAIL] $Name - $($_.Exception.Message)" -ForegroundColor Red
        $script:failed++
        return $null
    }
}

function Test-Field {
    param(
        [string]$Name,
        [object]$Object,
        [string]$FieldName
    )
    if ($Object.PSObject.Properties.Name -contains $FieldName) {
        Write-Host "[PASS] $Name" -ForegroundColor Green
        $script:passed++
        return $true
    } else {
        Write-Host "[FAIL] $Name - field '$FieldName' missing" -ForegroundColor Red
        $script:failed++
        return $false
    }
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   Phase 4 Feature Tests" -ForegroundColor Cyan
Write-Host "   TimeLapse Camera System v0.4.0" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Base URL: $BaseUrl" -ForegroundColor Gray
Write-Host ""

# ============================================
# Test 1: Basic Connectivity
# ============================================
Write-Host "--- Test 1: Basic Connectivity ---" -ForegroundColor Yellow

$health = Test-Endpoint "Health Check" "GET" "$BaseUrl/health"
if ($health -and $health.status -eq "ok") {
    Write-Host "   Status: $($health.status)" -ForegroundColor Gray
}

# ============================================
# Test 2: Camera and Profile Features
# ============================================
Write-Host ""
Write-Host "--- Test 2: Camera and Profile Features ---" -ForegroundColor Yellow

$cameras = Test-Endpoint "List Cameras" "GET" "$BaseUrl/api/v1/cameras"

if ($cameras -and $cameras.Count -gt 0) {
    $camera = $cameras[0]
    $uuid = $camera.uuid
    Write-Host "   Camera: $($camera.name)" -ForegroundColor Gray
    Write-Host "   UUID: $uuid" -ForegroundColor Gray

    # Test profile_token field
    Test-Field "Profile Token Field Exists" $camera "profile_token"
    if ($camera.profile_token) {
        Write-Host "   Profile Token: $($camera.profile_token)" -ForegroundColor Gray
    }

    # Test capture_profiles field
    Test-Field "Capture Profiles Field Exists" $camera "capture_profiles"

    # List ONVIF profiles
    $profiles = Test-Endpoint "List ONVIF Profiles" "GET" "$BaseUrl/api/v1/cameras/$uuid/profiles"
    if ($profiles) {
        Write-Host "   Found $($profiles.Count) ONVIF profile(s):" -ForegroundColor Gray
        foreach ($p in $profiles) {
            Write-Host "      - $($p.name) ($($p.token)): $($p.resolution)" -ForegroundColor Gray
        }
    }
} else {
    Write-Host "[SKIP] No cameras configured" -ForegroundColor Yellow
}

# ============================================
# Test 3: Camera Statistics (IP Change Detection)
# ============================================
Write-Host ""
Write-Host "--- Test 3: Camera Statistics ---" -ForegroundColor Yellow

if ($uuid) {
    $stats = Test-Endpoint "Get Camera Stats" "GET" "$BaseUrl/api/v1/cameras/$uuid/stats"
    if ($stats) {
        # Check for new Phase 4 fields
        Test-Field "Consecutive Failures Field" $stats "consecutive_failures"
        Test-Field "Reconnect Attempts Field" $stats "reconnect_attempts"

        Write-Host "   Total Captures: $($stats.total_captures)" -ForegroundColor Gray
        Write-Host "   Successful: $($stats.successful_captures)" -ForegroundColor Gray
        Write-Host "   Failed: $($stats.failed_captures)" -ForegroundColor Gray
        Write-Host "   Consecutive Failures: $($stats.consecutive_failures)" -ForegroundColor Gray
        Write-Host "   Reconnect Attempts: $($stats.reconnect_attempts)" -ForegroundColor Gray
        Write-Host "   Connected: $($stats.is_connected)" -ForegroundColor Gray
    }
}

# ============================================
# Test 4: Capture Functionality
# ============================================
Write-Host ""
Write-Host "--- Test 4: Capture Functionality ---" -ForegroundColor Yellow

if ($uuid) {
    $snapshot = Test-Endpoint "Take Snapshot" "POST" "$BaseUrl/api/v1/cameras/$uuid/snapshot"
    if ($snapshot) {
        Write-Host "   Filename: $($snapshot.filename)" -ForegroundColor Gray
    }

    Start-Sleep -Seconds 1

    $images = Test-Endpoint "List Images" "GET" "$BaseUrl/api/v1/cameras/$uuid/images"
    if ($images) {
        Write-Host "   Total Images: $($images.Count)" -ForegroundColor Gray
    }
}

# ============================================
# Test 5: Global Statistics
# ============================================
Write-Host ""
Write-Host "--- Test 5: Global Statistics ---" -ForegroundColor Yellow

$globalStats = Test-Endpoint "Global Stats" "GET" "$BaseUrl/api/v1/stats"
if ($globalStats) {
    Write-Host "   Total Captures: $($globalStats.total_captures)" -ForegroundColor Gray
    Write-Host "   Active Cameras: $($globalStats.active_cameras)" -ForegroundColor Gray
}

$storageStats = Test-Endpoint "Storage Stats" "GET" "$BaseUrl/api/v1/stats/storage"
if ($storageStats) {
    $sizeMB = [math]::Round($storageStats.total_size / 1MB, 2)
    Write-Host "   Total Images: $($storageStats.total_images)" -ForegroundColor Gray
    Write-Host "   Total Size: $sizeMB MB" -ForegroundColor Gray
}

# ============================================
# Results Summary
# ============================================
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "   Results Summary" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Passed: $passed" -ForegroundColor Green
Write-Host "Failed: $failed" -ForegroundColor $(if ($failed -gt 0) { "Red" } else { "Green" })
Write-Host ""

if ($failed -eq 0) {
    Write-Host "All Phase 4 tests PASSED!" -ForegroundColor Green
    Write-Host ""
    exit 0
} else {
    Write-Host "Some tests FAILED!" -ForegroundColor Red
    Write-Host ""
    exit 1
}
