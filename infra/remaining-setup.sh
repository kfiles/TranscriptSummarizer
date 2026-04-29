#!/usr/bin/env bash
# infra/remaining-setup.sh — runs the steps that failed in setup.sh after the
# GitHub repo was not yet connected to Cloud Build.
#
# Prerequisites: the repo has now been connected via Cloud Build → Triggers →
# Connect Repository in the Cloud Console.
#
# Run from the repo root:
#   PROJECT_ID=your-gcp-project bash infra/remaining-setup.sh

set -euo pipefail

PROJECT_ID="${PROJECT_ID:?Set PROJECT_ID environment variable}"
REGION="${REGION:-us-central1}"
CHANNEL_ID="UCGnv43oWpciURP-bTDc3GnA"
FIREBASE_PROJECT="miltonmeetingsummarizer"
CONTENT_BUCKET="${PROJECT_ID}-hugo-content"
PUBSUB_TOPIC="youtube-pipeline-trigger"
SA_NAME="transcript-summarizer"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud config set project "${PROJECT_ID}"

# ── Cloud Build trigger ────────────────────────────────────────────────────────
# The repo is connected via the 1st-gen GitHub App (not a 2nd-gen connection),
# so we read the owner/name from the existing push trigger rather than using
# `gcloud builds connections`, which only lists 2nd-gen connections.
echo "=== Finding connected GitHub repository ==="
GITHUB_OWNER=$(gcloud builds triggers describe transcript-summarizer-push \
  --project="${PROJECT_ID}" \
  --format="value(github.owner)")
GITHUB_REPO=$(gcloud builds triggers describe transcript-summarizer-push \
  --project="${PROJECT_ID}" \
  --format="value(github.name)")

if [[ -z "${GITHUB_OWNER}" || -z "${GITHUB_REPO}" ]]; then
  echo "ERROR: Could not read GitHub owner/repo from trigger transcript-summarizer-push."
  exit 1
fi
echo "  Repository: ${GITHUB_OWNER}/${GITHUB_REPO}"

echo "=== Creating Cloud Build trigger ==="
if gcloud builds triggers describe hugo-build-and-deploy --project="${PROJECT_ID}" &>/dev/null; then
  echo "  Trigger hugo-build-and-deploy already exists — skipping."
else
  gcloud builds triggers create pubsub \
    --name="hugo-build-and-deploy" \
    --topic="projects/${PROJECT_ID}/topics/${PUBSUB_TOPIC}" \
    --service-account="projects/${PROJECT_ID}/serviceAccounts/${SA_EMAIL}" \
    --repo-type=GITHUB \
    --repo="https://www.github.com/${GITHUB_OWNER}/${GITHUB_REPO}" \
    --build-config="cloudbuild.yaml" \
    --branch="main" \
    --substitutions="_CONTENT_BUCKET=${CONTENT_BUCKET},_FIREBASE_PROJECT=${FIREBASE_PROJECT}" \
    --project="${PROJECT_ID}"
fi

# ── Deploy Cloud Function ──────────────────────────────────────────────────────
echo "=== Deploying Cloud Function ==="
gcloud functions deploy youtube-webhook \
  --gen2 \
  --runtime=go124 \
  --region="${REGION}" \
  --source=. \
  --entry-point=YouTubeWebhook \
  --trigger-http \
  --allow-unauthenticated \
  --service-account="${SA_EMAIL}" \
  --timeout=540s \
  --memory=512Mi \
  --set-env-vars="HUGO_CONTENT_DIR=/tmp/hugo-content/minutes,GCS_BUCKET=${CONTENT_BUCKET},PUBSUB_PROJECT=${PROJECT_ID},PUBSUB_TOPIC=${PUBSUB_TOPIC}" \
  --set-secrets="\
MONGODB_URI=mongodb-uri:latest,\
CHATGPT_API_KEY=openai-api-key:latest,\
FACEBOOK_PAGE_ID=facebook-page-id:latest,\
FACEBOOK_PAGE_TOKEN=facebook-page-token:latest,\
YOUTUBE_API_KEY=youtube-api-key:latest" \
  --project="${PROJECT_ID}"

# ── Subscribe to YouTube PubSubHubbub ─────────────────────────────────────────
echo "=== Subscribing to YouTube push notifications ==="
FUNCTION_URL=$(gcloud functions describe youtube-webhook \
  --region="${REGION}" \
  --gen2 \
  --project="${PROJECT_ID}" \
  --format="value(serviceConfig.uri)")

TOPIC_URL="https://www.youtube.com/xml/feeds/videos.xml?channel_id=${CHANNEL_ID}"

curl -s -o /dev/null -w "Hub subscription response: %{http_code}\n" \
  -X POST https://pubsubhubbub.appspot.com/subscribe \
  --data-urlencode "hub.callback=${FUNCTION_URL}" \
  --data-urlencode "hub.topic=${TOPIC_URL}" \
  -d "hub.mode=subscribe" \
  -d "hub.verify=async"

# ── Cloud Scheduler — renew the PubSubHubbub subscription every 9 days ───────
echo "=== Creating Cloud Scheduler renewal job ==="
gcloud scheduler jobs create http renew-youtube-subscription \
  --location="${REGION}" \
  --schedule="0 9 */9 * *" \
  --uri="https://pubsubhubbub.appspot.com/subscribe" \
  --http-method=POST \
  --headers="Content-Type=application/x-www-form-urlencoded" \
  --message-body="hub.callback=${FUNCTION_URL}&hub.topic=${TOPIC_URL}&hub.mode=subscribe&hub.verify=async" \
  --project="${PROJECT_ID}" || \
gcloud scheduler jobs update http renew-youtube-subscription \
  --location="${REGION}" \
  --schedule="0 9 */9 * *" \
  --uri="https://pubsubhubbub.appspot.com/subscribe" \
  --http-method=POST \
  --headers="Content-Type=application/x-www-form-urlencoded" \
  --message-body="hub.callback=${FUNCTION_URL}&hub.topic=${TOPIC_URL}&hub.mode=subscribe&hub.verify=async" \
  --project="${PROJECT_ID}"

echo ""
echo "=== Setup complete ==="
echo "Function URL:       ${FUNCTION_URL}"
echo "Subscription topic: ${TOPIC_URL}"
echo ""
echo "Test the webhook end-to-end:"
echo "  curl -X POST ${FUNCTION_URL} \\"
echo "    -H 'Content-Type: application/atom+xml' \\"
echo "    --data-binary @infra/sample-notification.xml"
