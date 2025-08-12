#!/usr/bin/env bash
set -euo pipefail

# Builds and deploys the API to Cloud Run using Google Cloud Build (no local Docker required).

PROJECT_ID="${PROJECT_ID:-w40k-468120}"
REGION="${REGION:-europe-west1}"
REPO="${REPO:-containers}"
SERVICE="${SERVICE:-w40k-duel}"
IMAGE_URI="${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/${SERVICE}:latest"

echo "Project:   ${PROJECT_ID}"
echo "Region:    ${REGION}"
echo "Repo:      ${REPO}"
echo "Service:   ${SERVICE}"
echo "Image URI: ${IMAGE_URI}"

command -v gcloud >/dev/null || { echo "ERROR: gcloud CLI not found."; exit 1; }

echo "Setting project..."
gcloud config set project "${PROJECT_ID}" 1>/dev/null

echo "Enabling required APIs..."
gcloud services enable artifactregistry.googleapis.com run.googleapis.com cloudbuild.googleapis.com --quiet

echo "Ensuring Artifact Registry repo exists..."
if ! gcloud artifacts repositories describe "${REPO}" --location="${REGION}" 1>/dev/null 2>&1; then
  gcloud artifacts repositories create "${REPO}" \
    --repository-format=docker \
    --location="${REGION}" \
    --description="Container images for w40k-duel" --quiet
fi

# Submit a Cloud Build using the provided YAML

echo "Submitting Cloud Build..."
gcloud builds submit --config cloudbuild_api.yaml --substitutions _IMAGE_URI="${IMAGE_URI}" --quiet

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
