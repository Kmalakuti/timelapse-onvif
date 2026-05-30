# Development Dockerfile for TimeLapse
FROM golang:1.23-alpine

# Install build dependencies
RUN apk add --no-cache git make bash ffmpeg

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies
RUN go mod download || true

# Copy source code
COPY . .

# Entrypoint auto-resolves deps when go.sum is empty (common with volume mounts)
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose ports
EXPOSE 8000 5173

ENTRYPOINT ["/entrypoint.sh"]
CMD ["go", "run", "cmd/timelapse-server/main.go"]
