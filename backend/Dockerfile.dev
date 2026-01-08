# Development Dockerfile with hot reload using Air
FROM docker.1ms.run/library/golang:1.25-alpine

WORKDIR /app

# Install air for hot reload
RUN go install github.com/air-verse/air@latest

# Install dependencies for debugging
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Source code will be mounted as volume

# Expose port
EXPOSE 8080

# Use air for hot reload
CMD ["air", "-c", ".air.toml"]
