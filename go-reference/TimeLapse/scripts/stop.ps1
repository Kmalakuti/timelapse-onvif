# TimeLapse - Stop Script
# Usage: .\scripts\stop.ps1

Write-Host "TimeLapse - Stopping containers..." -ForegroundColor Cyan

# Stop and remove containers
docker-compose down --remove-orphans

Write-Host "Containers stopped." -ForegroundColor Green
