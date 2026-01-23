# Development Dockerfile for Relay with hot reload using Air
FROM docker.1ms.run/library/golang:1.25-alpine

# Install air for hot reload
RUN go install github.com/air-verse/air@latest

# Install dependencies for debugging
RUN apk add --no-cache git ca-certificates tzdata

# Copy relay module
WORKDIR /app
COPY relay/go.mod relay/go.sum ./
RUN go mod download

# Source code will be mounted as volume

# Expose port
EXPOSE 8090

# Use air for hot reload
CMD ["air", "-c", ".air.toml"]
