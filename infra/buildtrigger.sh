#!/usr/bin/env bash
# infra/buildtrigger.sh — reinstall hub pubsub build trigger
#
# Prerequisites:
#   gcloud auth login && gcloud auth application-default login
#
# Run from the repo root:
#   PROJECT_ID=your-gcp-project bash infra/buildtrigger.sh

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

# Allow Cloud Build SA to deploy Firebase Hosting and read the firebase-ci-token secret.
# Secret access is granted at the secret level (not project level) to avoid the
# project's conditional IAM policy requiring an explicit condition on every binding.
PROJECT_NUMBER=$(gcloud projects describe "${PROJECT_ID}" --format='value(projectNumber)')
CLOUDBUILD_SA="${PROJECT_NUMBER}@cloudbuild.gserviceaccount.com"


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
  echo "Removing existing build trigger."
  gcloud builds triggers delete "hugo-build-and-deploy"
fi

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
