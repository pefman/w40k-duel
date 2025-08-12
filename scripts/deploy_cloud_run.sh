#!/usr/bin/env bash
set -euo pipefail

# Deploys the API service to Google Cloud Run using Docker + Artifact Registry.
# Requirements: gcloud CLI, Docker, authenticated gcloud (gcloud auth login), project access.

PROJECT_ID="${PROJECT_ID:-w40k-468120}"
REGION="${REGION:-europe-west1}"
REPO="${REPO:-containers}"
SERVICE="${SERVICE:-w40k-api}"
IMAGE_URI="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${SERVICE}:latest"

echo "Project:   ${PROJECT_ID}"
echo "Region:    ${REGION}"
echo "Repo:      ${REPO}"
echo "Service:   ${SERVICE}"
echo "Image URI: ${IMAGE_URI}"

# Sanity checks
command -v gcloud >/dev/null || { echo "ERROR: gcloud CLI not found."; exit 1; }
command -v docker >/dev/null || { echo "ERROR: Docker not found (required for local container build)."; exit 1; }

# Configure gcloud
echo "Setting project..."
gcloud config set project "${PROJECT_ID}" 1>/dev/null

# Enable required services (idempotent)
echo "Enabling APIs (Artifact Registry, Cloud Run)..."
gcloud services enable artifactregistry.googleapis.com run.googleapis.com --quiet

# Create Artifact Registry repo if missing (idempotent)
echo "Ensuring Artifact Registry repo exists..."
if ! gcloud artifacts repositories describe "${REPO}" --location="${REGION}" 1>/dev/null 2>&1; then
  gcloud artifacts repositories create "${REPO}" \
    --repository-format=docker \
    --location="${REGION}" \
    --description="Container images for w40k-duel" --quiet
fi

# Configure Docker to auth with AR
echo "Configuring Docker auth for Artifact Registry..."
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet

# Build image
echo "Building Docker image..."
docker build -f Dockerfile.api -t "${IMAGE_URI}" .

# Push image
echo "Pushing image to Artifact Registry..."
docker push "${IMAGE_URI}"

# Deploy to Cloud Run
echo "Deploying to Cloud Run (${SERVICE})..."
gcloud run deploy "${SERVICE}" \
  --image="${IMAGE_URI}" \
  --region="${REGION}" \
  --allow-unauthenticated \
  --port=8080 \
  --quiet

URL=$(gcloud run services describe "${SERVICE}" --region "${REGION}" --format='value(status.url)')
echo "Deployed: ${URL}"
