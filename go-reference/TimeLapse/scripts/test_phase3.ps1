# Phase 3 Testing Script for Windows PowerShell
# Run this on the testing machine with Docker Desktop

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  Phase 3 Testing - TimeLapse Camera" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Test results
$results = @()
$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$resultsFile = "test_results_$timestamp.txt"

function Log-Result {
    param($TestName, $Status, $Details)

    $script:results += [PSCustomObject]@{
        Test = $TestName
        Status = $Status
        Details = $Details
    }

    if ($Status -eq "PASS") {
        Write-Host "[PASS] " -ForegroundColor Green -NoNewline
        Write-Host $TestName
    } elseif ($Status -eq "SKIP") {
        Write-Host "[SKIP] " -ForegroundColor Yellow -NoNewline
        Write-Host "$TestName - $Details"
    } else {
        Write-Host "[FAIL] " -ForegroundColor Red -NoNewline
        Write-Host "$TestName - $Details"
    }
}

# Step 1: Build Docker image
Write-Host ""
Write-Host "Step 1: Building Docker image..." -ForegroundColor Yellow
docker-compose build
if ($LASTEXITCODE -ne 0) {
    Write-Host "Docker build failed!" -ForegroundColor Red
    exit 1
}

# Step 2: Run Unit Tests
Write-Host ""
Write-Host "Step 2: Running Unit Tests..." -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Cyan

$unitTestOutput = docker-compose run --rm timelapse-dev go test ./internal/... -v 2>&1
$unitTestOutput | Out-File -FilePath "unit_tests.log" -Encoding UTF8

if ($LASTEXITCODE -eq 0) {
    Log-Result "Unit Tests" "PASS" "All tests passed"
} else {
    Log-Result "Unit Tests" "FAIL" "See unit_tests.log for details"
}

# Step 3: Start API Server
Write-Host ""
Write-Host "Step 3: Starting API Server..." -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Cyan

# Stop any existing containers first
docker-compose down 2>$null

# Start in background
docker-compose up -d timelapse-dev
Write-Host "Waiting 15 seconds for server to start..."
Start-Sleep -Seconds 15

# Step 4: Test API Endpoints
Write-Host ""
Write-Host "Step 4: Testing API Endpoints..." -ForegroundColor Yellow
Write-Host "==========================================" -ForegroundColor Cyan

# Test 1: Health check
Write-Host "Test 1: Health Check"
try {
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:8000/health" -Method Get -ErrorAction Stop
    if ($health.status -eq "ok") {
        Log-Result "Health Check" "PASS" "status=ok"
    } else {
        Log-Result "Health Check" "FAIL" "Unexpected response: $($health | ConvertTo-Json -Compress)"
    }
} catch {
    Log-Result "Health Check" "FAIL" $_.Exception.Message
}

# Test 2: List cameras
Write-Host "Test 2: List Cameras"
$cameraUUID = $null
try {
    $cameras = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras" -Method Get -ErrorAction Stop
    if ($cameras.cameras -and $cameras.cameras.Count -ge 0) {
        Log-Result "List Cameras" "PASS" "Found $($cameras.total) camera(s)"
        if ($cameras.cameras.Count -gt 0) {
            $cameraUUID = $cameras.cameras[0].uuid
            Write-Host "       Camera UUID: $cameraUUID" -ForegroundColor Gray
        }
    } else {
        Log-Result "List Cameras" "FAIL" "Unexpected response structure"
    }
} catch {
    Log-Result "List Cameras" "FAIL" $_.Exception.Message
}

# Test 3: Get statistics
Write-Host "Test 3: Get Statistics"
try {
    $stats = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/stats" -Method Get -ErrorAction Stop
    if ($null -ne $stats.cameras) {
        Log-Result "Get Statistics" "PASS" "Cameras: $($stats.cameras.total), Captures: $($stats.capture.total_captures)"
    } else {
        Log-Result "Get Statistics" "FAIL" "Unexpected response structure"
    }
} catch {
    Log-Result "Get Statistics" "FAIL" $_.Exception.Message
}

# Test 4: Probe camera
Write-Host "Test 4: Probe Camera"
try {
    $probeBody = @{
        ip = "192.168.200.13"
        port = 80
        username = "admin"
        password = $env:TIMELAPSE_TEST_CAMERA_PASSWORD
    } | ConvertTo-Json

    $probe = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/discovery/probe" `
        -Method Post `
        -ContentType "application/json" `
        -Body $probeBody `
        -ErrorAction Stop

    if ($probe.success -eq $true) {
        Log-Result "Probe Camera" "PASS" "Found: $($probe.device.manufacturer) $($probe.device.model)"
    } else {
        Log-Result "Probe Camera" "FAIL" $probe.error
    }
} catch {
    Log-Result "Probe Camera" "FAIL" $_.Exception.Message
}

# Test 5: List profiles (requires camera UUID)
Write-Host "Test 5: List Profiles"
if ($cameraUUID) {
    try {
        $profiles = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras/$cameraUUID/profiles" -Method Get -ErrorAction Stop
        if ($profiles.profiles) {
            Log-Result "List Profiles" "PASS" "Found $($profiles.profiles.Count) profile(s)"
            foreach ($p in $profiles.profiles) {
                $active = if ($p.is_active) { " (ACTIVE)" } else { "" }
                Write-Host "       - $($p.name): $($p.resolution)$active" -ForegroundColor Gray
            }
        } else {
            Log-Result "List Profiles" "FAIL" "No profiles in response"
        }
    } catch {
        Log-Result "List Profiles" "FAIL" $_.Exception.Message
    }
} else {
    Log-Result "List Profiles" "SKIP" "No camera UUID available"
}

# Test 6: Take snapshot
Write-Host "Test 6: Take Snapshot"
if ($cameraUUID) {
    try {
        $snapshot = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras/$cameraUUID/snapshot" `
            -Method Post `
            -ErrorAction Stop

        if ($snapshot.success -eq $true) {
            Log-Result "Take Snapshot" "PASS" "Saved: $($snapshot.filename) ($($snapshot.size) bytes)"
        } else {
            Log-Result "Take Snapshot" "FAIL" "success=false"
        }
    } catch {
        Log-Result "Take Snapshot" "FAIL" $_.Exception.Message
    }
} else {
    Log-Result "Take Snapshot" "SKIP" "No camera UUID available"
}

# Test 7: List images
Write-Host "Test 7: List Images"
if ($cameraUUID) {
    try {
        $images = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/cameras/$cameraUUID/images?limit=10" -Method Get -ErrorAction Stop
        if ($null -ne $images.images) {
            Log-Result "List Images" "PASS" "Found $($images.total) image(s)"
        } else {
            Log-Result "List Images" "FAIL" "Unexpected response structure"
        }
    } catch {
        Log-Result "List Images" "FAIL" $_.Exception.Message
    }
} else {
    Log-Result "List Images" "SKIP" "No camera UUID available"
}

# Test 8: Storage stats
Write-Host "Test 8: Storage Statistics"
try {
    $storageStats = Invoke-RestMethod -Uri "http://127.0.0.1:8000/api/v1/stats/storage" -Method Get -ErrorAction Stop
    if ($null -ne $storageStats.total_images) {
        Log-Result "Storage Stats" "PASS" "Images: $($storageStats.total_images), Size: $($storageStats.total_size) bytes"
    } else {
        Log-Result "Storage Stats" "FAIL" "Unexpected response structure"
    }
} catch {
    Log-Result "Storage Stats" "FAIL" $_.Exception.Message
}

# Summary
Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "  Test Summary" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

$passed = ($results | Where-Object { $_.Status -eq "PASS" }).Count
$failed = ($results | Where-Object { $_.Status -eq "FAIL" }).Count
$skipped = ($results | Where-Object { $_.Status -eq "SKIP" }).Count
$total = $results.Count

Write-Host ""
Write-Host "Passed:  $passed / $total" -ForegroundColor Green
Write-Host "Failed:  $failed / $total" -ForegroundColor $(if ($failed -gt 0) { "Red" } else { "Green" })
Write-Host "Skipped: $skipped / $total" -ForegroundColor Yellow
Write-Host ""

# Save results to file
$results | Format-Table -AutoSize | Out-String | Out-File -FilePath $resultsFile -Encoding UTF8
Write-Host "Results saved to: $resultsFile"
Write-Host "Unit test log saved to: unit_tests.log"

# Step 5: Cleanup option
Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
$cleanup = Read-Host "Stop Docker containers? (y/n)"
if ($cleanup -eq "y") {
    docker-compose down
    Write-Host "Containers stopped." -ForegroundColor Green
} else {
    Write-Host "Containers still running. Stop with: docker-compose down" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Done! Copy results to PHASE3_COMPLETE.md" -ForegroundColor Cyan
