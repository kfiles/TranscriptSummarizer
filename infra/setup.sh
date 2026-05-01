#!/usr/bin/env bash
# infra/setup.sh — one-time infrastructure setup for the YouTube webhook pipeline.
#
# Prerequisites:
#   gcloud auth login && gcloud auth application-default login
#   firebase login --ci  (to get FIREBASE_CI_TOKEN below)
#
# Run from the repo root:
#   PROJECT_ID=your-gcp-project bash infra/setup.sh

set -euo pipefail

PROJECT_ID="${PROJECT_ID:?Set PROJECT_ID environment variable}"
REGION="${REGION:-us-central1}"
CHANNEL_ID="UCGnv43oWpciURP-bTDc3GnA"
FIREBASE_PROJECT="miltonmeetingsummarizer"
CONTENT_BUCKET="${PROJECT_ID}-hugo-content"
PUBSUB_TOPIC="youtube-pipeline-trigger"
SA_NAME="transcript-summarizer"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

echo "=== Configuring project ${PROJECT_ID} ==="
gcloud config set project "${PROJECT_ID}"

# ── Enable required APIs ──────────────────────────────────────────────────────
echo "=== Enabling APIs ==="
gcloud services enable \
  cloudfunctions.googleapis.com \
  cloudbuild.googleapis.com \
  run.googleapis.com \
  secretmanager.googleapis.com \
  pubsub.googleapis.com \
  storage.googleapis.com \
  cloudscheduler.googleapis.com \
  youtube.googleapis.com

# ── Service account ───────────────────────────────────────────────────────────
echo "=== Creating service account ==="
gcloud iam service-accounts create "${SA_NAME}" \
  --display-name="Transcript Summarizer Function" \
  --project="${PROJECT_ID}" || true   # ignore if already exists

# Grant the function SA access to each secret individually.
# Project-level secretmanager.secretAccessor is blocked by the project's conditional
# IAM policy, so we bind at the secret resource level instead.
grant_secret_access() {
  gcloud secrets add-iam-policy-binding "$1" \
    --member="serviceAccount:${SA_EMAIL}" \
    --role="roles/secretmanager.secretAccessor" \
    --project="${PROJECT_ID}"
}
# Secrets are created later in this script; this function is called after creation.

# Allow the function SA to write to GCS
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/storage.objectCreator" \
  --condition=None

# Allow the function SA to publish to Pub/Sub
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/pubsub.publisher" \
  --condition=None

# Allow the function SA to write Cloud Build logs to Cloud Logging.
# Required because this SA is the build runner for the hugo-build-and-deploy trigger.
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/logging.logWriter" \
  --condition=None

# Allow the function SA to read/write the meetingtranscripts Firestore database
# (MongoDB compatibility mode). Condition scopes access to that database only.
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/datastore.user" \
  --condition="expression=resource.name == 'projects/${PROJECT_ID}/databases/meetingtranscripts',title=meetingtranscripts-writer-role"

# Allow the function SA to use Firebase Admin SDK (Firestore, Auth, etc.)
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/firebase.admin" \
  --condition=None

# Allow Cloud Build SA to deploy Firebase Hosting and read the firebase-ci-token secret.
# Secret access is granted at the secret level (not project level) to avoid the
# project's conditional IAM policy requiring an explicit condition on every binding.
PROJECT_NUMBER=$(gcloud projects describe "${PROJECT_ID}" --format='value(projectNumber)')
CLOUDBUILD_SA="${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com"
gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${CLOUDBUILD_SA}" \
  --role="roles/firebasehosting.admin" \
  --condition=None
gcloud secrets add-iam-policy-binding "firebase-ci-token" \
  --member="serviceAccount:${CLOUDBUILD_SA}" \
  --role="roles/secretmanager.secretAccessor" \
  --project="${PROJECT_ID}"
gcloud secrets add-iam-policy-binding "mongodb-uri" \
  --member="serviceAccount:${CLOUDBUILD_SA}" \
  --role="roles/secretmanager.secretAccessor" \
  --project="${PROJECT_ID}"

# ── GCS bucket for Hugo content ───────────────────────────────────────────────
echo "=== Creating GCS content bucket ==="
gcloud storage buckets create "gs://${CONTENT_BUCKET}" \
  --location="${REGION}" \
  --uniform-bucket-level-access || true

# ── Pub/Sub topic ─────────────────────────────────────────────────────────────
echo "=== Creating Pub/Sub topic ==="
gcloud pubsub topics create "${PUBSUB_TOPIC}" --project="${PROJECT_ID}" || true

# ── Secrets ───────────────────────────────────────────────────────────────────
# For each secret, the script checks whether it exists and creates it if not.
# To update a secret value: gcloud secrets versions add SECRET_NAME --data-file=-
echo "=== Creating Secret Manager secrets ==="
create_secret() {
  local name="$1"
  local prompt="$2"
  if ! gcloud secrets describe "${name}" --project="${PROJECT_ID}" &>/dev/null; then
    echo -n "${prompt}: "
    read -rs value
    echo
    echo -n "${value}" | gcloud secrets create "${name}" \
      --data-file=- \
      --replication-policy=automatic \
      --project="${PROJECT_ID}"
  else
    echo "  Secret ${name} already exists — skipping."
  fi
}

create_secret "mongodb-uri"           "MongoDB connection string (MONGODB_URI)"
create_secret "openai-api-key"        "OpenAI API key (CHATGPT_API_KEY)"
create_secret "facebook-page-id"      "Facebook Page ID"
create_secret "facebook-page-token"   "Facebook Page access token"
create_secret "youtube-api-key"       "YouTube Data API key (public-data API key, no OAuth)"
create_secret "supadata-api-key"      "Supadata API key (transcript provider)"
create_secret "transcriptapi-api-key" "TranscriptAPI.com API key (transcript provider)"
create_secret "firebase-ci-token"     "Firebase CI token (from: firebase login:ci)"

# Grant the function SA accessor rights on each secret it needs at runtime.
for SECRET in mongodb-uri openai-api-key facebook-page-id facebook-page-token youtube-api-key supadata-api-key transcriptapi-api-key firebase-api-token; do
  grant_secret_access "${SECRET}"
done

# ── Cloud Build trigger (Pub/Sub → Hugo build + Firebase deploy) ──────────────
echo "=== Creating Cloud Build trigger ==="
# The trigger fires whenever the pipeline function publishes to PUBSUB_TOPIC.
GITHUB_OWNER=$(gcloud builds triggers describe transcript-summarizer-push \
  --project="${PROJECT_ID}" \
  --format="value(github.owner)")
GITHUB_REPO=$(gcloud builds triggers describe transcript-summarizer-push \
  --project="${PROJECT_ID}" \
  --format="value(github.name)")
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

# ── Deploy Cloud Function ─────────────────────────────────────────────────────
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
  --set-env-vars="HUGO_CONTENT_DIR=/tmp/hugo-content/minutes,GCS_BUCKET=${CONTENT_BUCKET},PUBSUB_PROJECT=${PROJECT_ID},PUBSUB_TOPIC=${PUBSUB_TOPIC},TRANSCRIPT_PROVIDER=transcriptapi,PROJECT_ID=${PROJECT_ID}" \
  --set-secrets="\
MONGODB_URI=mongodb-uri:latest,\
CHATGPT_API_KEY=openai-api-key:latest,\
FACEBOOK_PAGE_ID=facebook-page-id:latest,\
FACEBOOK_PAGE_TOKEN=facebook-page-token:latest,\
YOUTUBE_API_KEY=youtube-api-key:latest,\
SUPADATA_API_KEY=supadata-api-key:latest,\
TRANSCRIPTAPI_API_KEY=transcriptapi-api-key:latest" \
  --project="${PROJECT_ID}"

# ── Subscribe to YouTube PubSubHubbub ─────────────────────────────────────────
echo "=== Subscribing to YouTube push notifications ==="
FUNCTION_URL=$(gcloud functions describe youtube-webhook \
  --region="${REGION}" \
  --gen2 \
  --project="${PROJECT_ID}" \
  --format='value(serviceConfig.uri)')

TOPIC_URL="https://www.youtube.com/xml/feeds/videos.xml?channel_id=${CHANNEL_ID}"

curl -s -o /dev/null -w "Hub subscription response: %{http_code}\n" \
  -X POST https://pubsubhubbub.appspot.com/subscribe \
  --data-urlencode "hub.callback=${FUNCTION_URL}" \
  --data-urlencode "hub.topic=${TOPIC_URL}" \
  -d "hub.mode=subscribe" \
  -d "hub.verify=async"

# ── Cloud Scheduler — renew the PubSubHubbub subscription every 9 days ───────
# YouTube subscriptions expire after ~10 days; renewing at day 9 ensures continuity.
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
echo "Function URL: ${FUNCTION_URL}"
echo "Subscription topic: ${TOPIC_URL}"
echo ""
echo "Next steps:"
echo "  1. Connect your GitHub repo in Cloud Build (Cloud Console → Cloud Build → Triggers)"
echo "     and confirm the trigger 'hugo-build-and-deploy' points to the correct repo."
echo "  2. Confirm the Cloud Scheduler job fires correctly:"
echo "     gcloud scheduler jobs run renew-youtube-subscription --location=${REGION}"
echo "  3. Test the webhook end-to-end by sending a sample Atom payload:"
echo "     curl -X POST ${FUNCTION_URL} \\"
echo "       -H 'Content-Type: application/atom+xml' \\"
echo "       --data-binary @infra/sample-notification.xml"
