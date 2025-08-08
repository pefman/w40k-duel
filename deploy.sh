#!/bin/bash

# W40K Duel Deployment Script
# This script builds the Docker image, pushes it to GCR, and updates Cloud Run

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ID="w40k-468120"
SERVICE_NAME="w40k-duel"
REGION="europe-west1"
IMAGE_NAME="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"

echo -e "${BLUE}🚀 Starting W40K Duel deployment...${NC}"

# Stop any running local instances
echo -e "${YELLOW}🛑 Stopping local instances...${NC}"
pkill -f "go run" || true
pkill -f "w40k-duel" || true

# Build Docker image
echo -e "${YELLOW}🔨 Building Docker image...${NC}"
docker build -t ${SERVICE_NAME} .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Docker image built successfully${NC}"
else
    echo -e "${RED}❌ Docker build failed${NC}"
    exit 1
fi

# Tag for GCR
echo -e "${YELLOW}🏷️  Tagging image for Google Container Registry...${NC}"
docker tag ${SERVICE_NAME} ${IMAGE_NAME}:latest

# Push to GCR
echo -e "${YELLOW}📤 Pushing image to Google Container Registry...${NC}"
docker push ${IMAGE_NAME}:latest

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Image pushed successfully${NC}"
else
    echo -e "${RED}❌ Image push failed${NC}"
    exit 1
fi

# Deploy to Cloud Run
echo -e "${YELLOW}🚢 Deploying to Cloud Run...${NC}"
gcloud run deploy ${SERVICE_NAME} \
    --image ${IMAGE_NAME}:latest \
    --region ${REGION} \
    --platform managed \
    --allow-unauthenticated \
    --memory 1Gi \
    --cpu 1 \
    --timeout 300s \
    --max-instances 10 \
    --min-instances 0

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Deployment successful!${NC}"
    
    # Get service URL
    SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region=${REGION} --format='value(status.url)')
    echo -e "${GREEN}🌐 Service URL: ${SERVICE_URL}${NC}"
    
    # Optional: Open in browser (uncomment if desired)
    # echo -e "${BLUE}🌐 Opening service in browser...${NC}"
    # xdg-open "${SERVICE_URL}" 2>/dev/null || open "${SERVICE_URL}" 2>/dev/null || true
    
else
    echo -e "${RED}❌ Deployment failed${NC}"
    exit 1
fi

echo -e "${GREEN}🎉 Deployment completed successfully!${NC}"
echo -e "${BLUE}📋 Summary:${NC}"
echo -e "   Project: ${PROJECT_ID}"
echo -e "   Service: ${SERVICE_NAME}"
echo -e "   Region: ${REGION}"
echo -e "   Image: ${IMAGE_NAME}:latest"
echo -e "   URL: ${SERVICE_URL}"
