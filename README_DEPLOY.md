# Deploying go40k duel to Google Cloud Run

This project has two services:
- API (CSV-backed data service)
- Game (WebSocket game server + embedded UI)

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
docker build -f Dockerfile.api -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-api:latest .
# Build Game
docker build -f Dockerfile.game -t ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-game:latest .

# Push
docker push ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-api:latest
docker push ${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-game:latest
```

## Deploy to Cloud Run

```bash
# API service
gcloud run deploy w40k-api \
  --image=${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-api:latest \
  --region=${REGION} \
  --allow-unauthenticated \
  --port=8080

# Get API URL
API_URL=$(gcloud run services describe w40k-api --region ${REGION} --format='value(status.url)')

# Game service (pass API URL)
gcloud run deploy w40k-game \
  --image=${REGION}-docker.pkg.dev/${PROJECT_ID}/${REPO}/w40k-game:latest \
  --region=${REGION} \
  --allow-unauthenticated \
  --port=8081 \
  --set-env-vars=DATA_API_BASE=${API_URL}

# Verify
open $(gcloud run services describe w40k-game --region ${REGION} --format='value(status.url)')
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
