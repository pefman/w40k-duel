#!/bin/bash

# W40K Duel - Data Fetcher and Server
# Downloads BattleScribe data and starts the game server

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}$1${NC}"
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_warning() {
    echo -e "${YELLOW}$1${NC}"
}

# Cleanup function
cleanup() {
    print_warning "🧹 Cleaning up temporary files..."
    rm -rf tmp/ *.log necron_*.json 2>/dev/null || true
}

# Set trap for cleanup on exit
trap cleanup EXIT

print_status "🎮 Warhammer 40K Duel - Data Pipeline"
print_status "====================================="

# Handle different commands
case "${1:-start}" in
    "clean")
        print_status "🧹 Cleaning up project..."
        cleanup
        rm -rf bin/ static/raw/ static/json/ w40k-duel 2>/dev/null || true
        print_success "✅ Project cleaned!"
        exit 0
        ;;
    "fetch")
        FORCE_FETCH=true
        ;;
    "start")
        FORCE_FETCH=false
        ;;
    *)
        echo "Usage: $0 [start|fetch|clean]"
        echo "  start: Start server (default)"
        echo "  fetch: Force data download"
        echo "  clean: Clean all generated files"
        exit 1
        ;;
esac

# Create directories
mkdir -p bin static

# Build binaries
print_status "🔨 Building binaries..."
go build -o bin/fetcher cmd/fetcher/main.go
go build -o w40k-duel .
print_success "✅ Build complete!"

# Fetch data if not present or if requested
if [ "$FORCE_FETCH" = true ] || [ ! -d "static/json" ] || [ -z "$(ls -A static/json 2>/dev/null)" ]; then
    print_status "📥 Fetching W40K data from BattleScribe..."
    ./bin/fetcher
    print_success "✅ Data fetch complete!"
    if [ -d "static/json" ]; then
        print_status "📊 $(ls static/json/*.json 2>/dev/null | wc -l) faction files generated"
        print_status "💾 $(du -sh static/json 2>/dev/null | cut -f1) total data size"
    fi
else
    if [ -d "static/json" ]; then
        print_status "📊 Using existing data: $(ls static/json/*.json 2>/dev/null | wc -l) faction files"
    fi
fi

echo ""
print_status "🚀 Starting W40K Duel server..."
print_success "🌐 Open http://localhost:8080 in your browser"
print_warning "📜 Press Ctrl+C to stop"
echo ""

# Start server
./w40k-duel
