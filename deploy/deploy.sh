#!/usr/bin/env bash
#
# Deploy WhatsWebService control-plane to Cloud Run in us-central1.
#
# Cloud Functions 2nd gen and Cloud Run share the same underlying runtime
# and billing model on GCP. We use `gcloud run deploy` because our shape
# (HTTP server with a chi router serving multiple routes) is a natural
# Cloud Run fit, not a single-handler Function.
#
# Required env vars when running:
#   GCP_PROJECT         your project id (e.g. whatdidyousay-prod)
#   TTS_CACHE_BUCKET    GCS bucket name for the TTS audio cache
#
# Optional:
#   SERVICE_NAME        defaults to "whats-control-plane"
#   REGION              defaults to "us-central1" (cheapest)
#   APPLE_TEAM_ID       set only if Apple Music endpoint should be enabled
#   APPLE_KEY_ID        ditto
#
# Secrets must already exist in Secret Manager — see deploy/README.md for the
# one-shot setup commands.

set -euo pipefail

: "${GCP_PROJECT:?set GCP_PROJECT to your project id}"
: "${TTS_CACHE_BUCKET:?set TTS_CACHE_BUCKET to your audio cache bucket name}"

SERVICE_NAME="${SERVICE_NAME:-whats-control-plane}"
REGION="${REGION:-us-central1}"
APPLE_TEAM_ID="${APPLE_TEAM_ID:-}"
APPLE_KEY_ID="${APPLE_KEY_ID:-}"

# Resolved at deploy time so the binary can read it as an env var.
SECRET_ENV_FLAGS=(
  "--update-secrets=OPENAI_API_KEY=openai-api-key:latest"
)
# Apple Music is optional; only mount its secret if the team/key are set.
if [[ -n "$APPLE_TEAM_ID" && -n "$APPLE_KEY_ID" ]]; then
  SECRET_ENV_FLAGS+=(
    "--update-secrets=/secrets/apple-music.p8=apple-music-p8:latest"
  )
fi

ENV_VARS=(
  "TTS_CACHE_BUCKET=${TTS_CACHE_BUCKET}"
  "OPENAI_TTS_MODEL=gpt-4o-mini-tts"
  "OPENAI_TTS_VOICE=nova"
  "OPENAI_TRANSLATE_MODEL=gpt-4o-mini"
  "OPENAI_TTS_FORMAT=mp3"
)
if [[ -n "$APPLE_TEAM_ID" && -n "$APPLE_KEY_ID" ]]; then
  ENV_VARS+=(
    "APPLE_TEAM_ID=${APPLE_TEAM_ID}"
    "APPLE_KEY_ID=${APPLE_KEY_ID}"
    "APPLE_MUSIC_PRIVATE_KEY_FILE=/secrets/apple-music.p8"
  )
fi

ENV_VARS_JOINED=$(IFS=,; echo "${ENV_VARS[*]}")

echo "Deploying ${SERVICE_NAME} to Cloud Run in ${REGION}..."
gcloud run deploy "${SERVICE_NAME}" \
  --project="${GCP_PROJECT}" \
  --region="${REGION}" \
  --source="../control-plane" \
  --platform=managed \
  --allow-unauthenticated \
  --port=8080 \
  --memory=256Mi \
  --cpu=1 \
  --min-instances=0 \
  --max-instances=10 \
  --concurrency=80 \
  --timeout=60 \
  --set-env-vars="${ENV_VARS_JOINED}" \
  "${SECRET_ENV_FLAGS[@]}"

echo
echo "Deployed. Service URL:"
gcloud run services describe "${SERVICE_NAME}" \
  --project="${GCP_PROJECT}" \
  --region="${REGION}" \
  --format='value(status.url)'
