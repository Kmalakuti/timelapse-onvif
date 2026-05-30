# TimeLapse - Start Script
# Usage: .\scripts\start.ps1

Write-Host "TimeLapse - Starting containers..." -ForegroundColor Cyan

# Clean up any stale containers/networks first
docker-compose down --remove-orphans 2>$null
docker network prune -f 2>$null

# Remove old containers if they exist with stale network references
docker rm -f timelapse-dev timelapse-frontend 2>$null

# Remove old node_modules volume if it exists (legacy cleanup)
docker volume rm timelapse_frontend-node-modules 2>$null

# Start both containers (builds frontend with nginx)
Write-Host "Building and starting containers..." -ForegroundColor Yellow
docker-compose up -d --build

# Wait for backend health check
Write-Host "Waiting for backend..." -ForegroundColor Yellow
$attempts = 0
$maxAttempts = 30
while ($attempts -lt $maxAttempts) {
    $health = docker inspect --format='{{.State.Health.Status}}' timelapse-dev 2>$null
    if ($health -eq "healthy") {
        Write-Host "  Backend: healthy" -ForegroundColor Green
        break
    }
    $attempts++
    Start-Sleep -Seconds 2
}

if ($attempts -eq $maxAttempts) {
    Write-Host "  Backend: timeout (may still be starting)" -ForegroundColor Yellow
}

# Wait for frontend health check
Write-Host "Waiting for frontend..." -ForegroundColor Yellow
$attempts = 0
$maxAttempts = 15
while ($attempts -lt $maxAttempts) {
    $health = docker inspect --format='{{.State.Health.Status}}' timelapse-frontend 2>$null
    if ($health -eq "healthy") {
        Write-Host "  Frontend: healthy" -ForegroundColor Green
        break
    }
    $attempts++
    Start-Sleep -Seconds 2
}

if ($attempts -eq $maxAttempts) {
    Write-Host "  Frontend: timeout (may still be starting)" -ForegroundColor Yellow
}

# Show status
Write-Host "`n=== Container Status ===" -ForegroundColor Cyan
docker-compose ps

Write-Host "`n=== Access URLs ===" -ForegroundColor Cyan
Write-Host "  Frontend UI:  http://localhost:5173" -ForegroundColor White
Write-Host "  Backend API:  http://localhost:8000" -ForegroundColor White
Write-Host "  Health Check: http://localhost:8000/health" -ForegroundColor White

Write-Host "`nTo view logs: docker-compose logs -f" -ForegroundColor Gray
Write-Host "To stop: .\scripts\stop.ps1" -ForegroundColor Gray
