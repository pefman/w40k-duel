# Warhammer 40K Duel - Makefile

.PHONY: all build fetcher server clean test run fetch help

# Default target
all: build

# Build all binaries
build:
	@echo "🔨 Building binaries..."
	@mkdir -p bin
	@go build -o w40k-duel .
	@go build -o bin/fetcher cmd/fetcher/main.go
	@echo "✅ Build complete!"

# Build only the fetcher
fetcher:
	@echo "🔨 Building fetcher..."
	@mkdir -p bin
	@go build -o bin/fetcher cmd/fetcher/main.go

# Build only the server
server:
	@echo "🔨 Building server..."
	@go build -o w40k-duel .

# Run tests
test:
	@echo "🧪 Running tests..."
	@go test ./...

# Fetch data
fetch: fetcher
	@echo "📥 Fetching W40K data..."
	@./bin/fetcher

# Run the application
run: build
	@echo "🚀 Starting W40K Duel server..."
	@./w40k-duel

# Start with script (handles data fetching automatically)
start:
	@./start.sh

# Force fetch new data and start
refresh:
	@./start.sh fetch

# Clean build artifacts
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/ w40k-duel tmp/ *.log necron_*.json
	@echo "✅ Clean complete!"

# Deep clean (including data)
clean-all: clean
	@echo "🧹 Deep cleaning..."
	@rm -rf static/raw/ static/json/
	@echo "✅ Deep clean complete!"

# Development setup
dev: build fetch
	@echo "🛠️  Development environment ready!"

# Show help
help:
	@echo "Warhammer 40K Duel - Available Commands:"
	@echo ""
	@echo "  make build     - Build all binaries"
	@echo "  make fetcher   - Build only the data fetcher"
	@echo "  make server    - Build only the game server"
	@echo "  make test      - Run tests"
	@echo "  make fetch     - Download W40K data"
	@echo "  make run       - Build and run server"
	@echo "  make start     - Start with automatic data handling"
	@echo "  make refresh   - Force fetch new data and start"
	@echo "  make clean     - Clean build artifacts"
	@echo "  make clean-all - Clean everything including data"
	@echo "  make dev       - Setup development environment"
	@echo "  make help      - Show this help"
	@echo ""
	@echo "Quick start: make start"
