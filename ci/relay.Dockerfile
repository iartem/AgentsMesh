# Build stage
# Build context should be project root (not relay/)
ARG REGISTRY=registry.corp.agentsmesh.ai
FROM ${REGISTRY}/library/golang:1.24-alpine AS builder

WORKDIR /app

# Use China Go proxy for faster module downloads
ENV GOPROXY=https://goproxy.cn,https://goproxy.io,direct
ENV GOSUMDB=sum.golang.google.cn

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY relay/go.mod relay/go.sum ./
RUN go mod download

# Copy source code
COPY relay/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/relay ./cmd/relay

# Final stage
ARG REGISTRY=registry.corp.agentsmesh.ai
FROM ${REGISTRY}/library/alpine:3.19

WORKDIR /app

# Install ca-certificates and tzdata
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S app && \
    adduser -u 1000 -S app -G app

# Copy binary from builder
COPY --from=builder /app/relay /app/relay

# Set ownership
RUN chown -R app:app /app

# Switch to non-root user
USER app

# Expose port
EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

# Run the relay server
ENTRYPOINT ["/app/relay"]
