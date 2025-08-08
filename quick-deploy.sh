#!/bin/bash
# Quick deployment script for W40K Duel

set -e
echo "🚀 Quick deploying W40K Duel..."

# Stop local instances
pkill -f "go run" || true

# Build, tag, and push
docker build -t w40k-duel .
docker tag w40k-duel gcr.io/w40k-468120/w40k-duel:latest
docker push gcr.io/w40k-468120/w40k-duel:latest

# Deploy
gcloud run deploy w40k-duel \
    --image gcr.io/w40k-468120/w40k-duel:latest \
    --region europe-west1 \
    --platform managed \
    --allow-unauthenticated

echo "✅ Deployment complete!"
