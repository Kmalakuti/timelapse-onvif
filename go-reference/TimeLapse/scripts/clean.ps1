# TimeLapse - Clean Script (Full Reset)
# Usage: .\scripts\clean.ps1
# This removes all containers, volumes, networks, and images for a fresh start

Write-Host "TimeLapse - Full cleanup..." -ForegroundColor Cyan
Write-Host "This will remove all containers, volumes, networks, and rebuild images." -ForegroundColor Yellow

# Stop all containers
docker-compose down --remove-orphans -v

# Remove any lingering containers
docker rm -f timelapse-dev timelapse-frontend 2>$null

# Prune networks
docker network prune -f

# Remove the built images to force rebuild
docker rmi timelapse-timelapse-dev 2>$null
docker rmi timelapse-timelapse-frontend 2>$null

# Remove legacy node_modules volume if it exists
docker volume rm timelapse_frontend-node-modules 2>$null

Write-Host "`nCleanup complete. Run .\scripts\start.ps1 to start fresh." -ForegroundColor Green
