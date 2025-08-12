# Deploying go40k duel to Google Cloud Run

This project currently deploys one service:
- API (CSV-backed data service and static UI)

## Prerequisites
- gcloud CLI installed and authenticated
- Google Cloud project ID: w40k-468120
- Artifact Registry API and Cloud Run API enabled
- A repository in Artifact Registry (region can be adjusted). This guide uses `europe-west1` and repo `containers`.

## Build and push images

```bash
# Set project and region
PROJECT_ID=w40k-468120
REGION=europe-west1
REPO=containers

# Configure gcloud
gcloud config set project "$PROJECT_ID"
gcloud auth configure-docker ${REGION}-docker.pkg.dev

# Build API
docker build -f Dockerfile.api -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-duel:latest .

# Push
docker push ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-duel:latest
```

## Deploy to Cloud Run

```bash
# API service
gcloud run deploy w40k-duel \
  --image=${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-duel:latest \
  --region=${REGION} \
  --allow-unauthenticated \
  --port=8080

# Verify
open $(gcloud run services describe w40k-duel --region ${REGION} --format='value(status.url)')
```

### Stable deployment using a service config

To ensure Cloud Run always deploys to the same service name/region/image path for the API, use the provided `cloudrun_api.yaml`:

```bash
# Replace (create/update) the service from YAML
gcloud run services replace cloudrun_api.yaml --region europe-west1
```

This keeps the service name `w40k-duel` consistent and pins the image path `europe-west1-docker.pkg.dev/w40k-468120/containers/w40k-duel:latest`.

Notes:
- Both services respect Cloud Run's `PORT` env var. Game also reads `DATA_API_BASE`.
- If using a different region or repo name, adjust variables accordingly.
- WebSockets are supported by Cloud Run.
