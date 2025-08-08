# Multi-stage Docker build for W40K Duel
# Stage 1: Build the Go application
FROM golang:1.24-alpine AS builder

# Set working directory
WORKDIR /app

# Install dependencies for building
RUN apk add --no-cache git ca-certificates

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the fetcher binary
RUN go build -o bin/fetcher cmd/fetcher/main.go

# Build the main application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o w40k-duel .

# Stage 2: Fetch W40K data
FROM builder AS data-fetcher

# Create directories for data
RUN mkdir -p static/json static/raw

# Run the fetcher to download and parse BattleScribe data
RUN ./bin/fetcher

# Stage 3: Final runtime image
FROM alpine:latest

# Install CA certificates for HTTPS requests (if needed for fallback API)
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/w40k-duel .

# Copy the processed data from data-fetcher stage
COPY --from=data-fetcher /app/static/json ./static/json

# Expose port (Cloud Run will set PORT environment variable)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:${PORT:-8080}/health || exit 1

# Run the application
CMD ["./w40k-duel"]
