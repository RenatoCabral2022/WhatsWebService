# Deploy — control-plane on Cloud Run (us-central1)

This directory contains the one-shot deployment artifacts for the
control-plane on GCP.

## Why Cloud Run, not Cloud Functions

Our control-plane is an HTTP server with a chi router serving multiple
routes (`/v1/tts/enunciate`, `/v1/music/apple/developer-token`,
`/healthz`). Cloud Functions 2nd gen is shaped for single-handler
functions; **Cloud Run is the correct target for multi-route HTTP servers
on GCP**. Under the hood, Cloud Functions 2nd gen *is* Cloud Run — same
runtime, same billing, same scale-to-zero. Using `gcloud run deploy`
directly skips one indirection without changing the cost model.

## One-time setup

Run these once per GCP project. Adjust `GCP_PROJECT` and `TTS_CACHE_BUCKET`.

```bash
export GCP_PROJECT=whatdidyousay-prod
export REGION=us-central1
export TTS_CACHE_BUCKET=${GCP_PROJECT}-tts-cache

# 1. Enable required APIs.
gcloud services enable run.googleapis.com cloudbuild.googleapis.com artifactregistry.googleapis.com secretmanager.googleapis.com storage.googleapis.com --project=${GCP_PROJECT}

# 2. Artifact Registry for container images.
gcloud artifacts repositories create whats --repository-format=docker --location=${REGION} --project=${GCP_PROJECT}

# 3. TTS audio cache bucket. Standard storage, lifecycle to Nearline after 30d.
gcloud storage buckets create gs://${TTS_CACHE_BUCKET} --project=${GCP_PROJECT} --location=${REGION} --uniform-bucket-level-access

cat > /tmp/lifecycle.json <<'EOF'
{
  "lifecycle": {
    "rule": [
      {
        "action": { "type": "SetStorageClass", "storageClass": "NEARLINE" },
        "condition": { "age": 30, "matchesStorageClass": ["STANDARD"] }
      }
    ]
  }
}
EOF
gcloud storage buckets update gs://${TTS_CACHE_BUCKET} --lifecycle-file=/tmp/lifecycle.json

# 4. Store secrets in Secret Manager. Have your OpenAI key and Apple .p8
#    in local files first.
echo -n "sk-proj-xxxxxxxxxx" | gcloud secrets create openai-api-key --project=${GCP_PROJECT} --replication-policy=automatic --data-file=-

# Skip the next step if you're not deploying Apple Music support.
gcloud secrets create apple-music-p8 --project=${GCP_PROJECT} --replication-policy=automatic --data-file=./control-plane/secrets/AuthKey_XXXXXXXXXX.p8

# 5. Grant the default Cloud Run service account access to the secrets and
#    the cache bucket. (The service account is created the first time you
#    deploy; for fresh projects, deploy once with `--no-traffic` first, then
#    grant, then re-deploy.)
PROJECT_NUMBER=$(gcloud projects describe ${GCP_PROJECT} --format='value(projectNumber)')
SA="${PROJECT_NUMBER}-compute@developer.gserviceaccount.com"

gcloud secrets add-iam-policy-binding openai-api-key --project=${GCP_PROJECT} --member="serviceAccount:${SA}" --role=roles/secretmanager.secretAccessor

gcloud secrets add-iam-policy-binding apple-music-p8 --project=${GCP_PROJECT} --member="serviceAccount:${SA}" --role=roles/secretmanager.secretAccessor || true

gcloud storage buckets add-iam-policy-binding gs://${TTS_CACHE_BUCKET} --member="serviceAccount:${SA}" --role=roles/storage.objectAdmin
```

## Deploy

### Option A — one-shot from your laptop

```bash
export GCP_PROJECT=whatdidyousay-prod
export TTS_CACHE_BUCKET=${GCP_PROJECT}-tts-cache
# Optional Apple Music: set these only if Apple Music endpoint is needed.
# export APPLE_TEAM_ID=XXXXXXXXXX
# export APPLE_KEY_ID=XXXXXXXXXX

cd deploy && bash deploy.sh
```

The script reads from `gcloud config` for auth, builds the image from
`control-plane/Dockerfile`, pushes to Artifact Registry, deploys to Cloud
Run with `min-instances=0` (scale-to-zero), and prints the service URL.

### Option B — Cloud Build (CI/CD)

```bash
gcloud builds submit --project=${GCP_PROJECT} --config=deploy/cloudbuild.yaml --substitutions=_TTS_CACHE_BUCKET=${TTS_CACHE_BUCKET},_APPLE_TEAM_ID=${APPLE_TEAM_ID:-},_APPLE_KEY_ID=${APPLE_KEY_ID:-} .
```

For continuous deploys on push to main, create a Cloud Build trigger
pointing at this file with the same substitutions.

## Verify

```bash
SERVICE_URL=$(gcloud run services describe whats-control-plane --project=${GCP_PROJECT} --region=${REGION} --format='value(status.url)')

# Health check
curl -sS "${SERVICE_URL}/healthz"

# TTS smoke test (no translation, EN only). First call cold-starts the
# function — expect 1-3s. Second call hits the cache — expect <500ms.
curl -X POST "${SERVICE_URL}/v1/tts/enunciate" -H 'Content-Type: application/json' -d '{"text":"hello from cloud run","sourceLanguage":"en"}' -o /tmp/out.mp3 -w '\nTTFB=%{time_starttransfer}s total=%{time_total}s\n'

# Verify the cache landed in GCS.
gcloud storage ls gs://${TTS_CACHE_BUCKET}/tts/
```

## Point the mobile app at it

```bash
# WHATS app, when rebuilding the iOS/Android binary:
EXPO_PUBLIC_WHATS_SERVICE_URL=${SERVICE_URL}/v1 npm run ios
```

For TestFlight / Play Store builds, set the env in `eas.json` or the
build config.

## Expected cost (low traffic, ~1000 requests/day)

| Item | ~Monthly |
|---|---|
| Cloud Run invocations + CPU/RAM | $0.50 |
| GCS storage + egress | $0.50 |
| Secret Manager (3 secrets) | $0.06 |
| **Total GCP** | **~$1–2** |
| OpenAI usage (separate) | bounded by Project cap |

Break-even vs an always-on VM is roughly >5,000 requests/day. Below that,
scale-to-zero wins.

## Rollback

```bash
# List recent revisions.
gcloud run revisions list --project=${GCP_PROJECT} --region=${REGION} --service=whats-control-plane

# Send 100% traffic to a previous revision.
gcloud run services update-traffic whats-control-plane --project=${GCP_PROJECT} --region=${REGION} --to-revisions=<revision-name>=100
```

## What this does NOT deploy

- `webrtc-gateway/` — for local-file mode only, stays on local docker compose.
- `inference/asr/` (Whisper + NLLB) — same.
- `inference/tts/` (Piper) — same.
- The mobile client's `source === 'local'` branch will fail in production
  (no backend reachable for ASR). Either guard it with a feature flag or
  hide the local-file picker in production builds.
