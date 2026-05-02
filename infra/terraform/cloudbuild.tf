# Pub/Sub-triggered build: syncs Hugo content from GCS and deploys to Firebase Hosting.
resource "google_cloudbuild_trigger" "hugo_build_and_deploy" {
  project  = var.project_id
  name     = "hugo-build-and-deploy"
  location = "global"

  service_account = "projects/${var.project_id}/serviceAccounts/${google_service_account.transcript_summarizer.email}"

  pubsub_config {
    topic = google_pubsub_topic.pipeline_trigger.id
  }

  # Source repository for checkout during builds.
  source_to_build {
    uri       = "https://github.com/${var.github_owner}/${var.github_repo}"
    ref       = "refs/heads/main"
    repo_type = "GITHUB"
  }

  # References cloudbuild.yaml from the repo root.
  # Note: live state has inline build steps (gcloud inlined them at creation time).
  # Switching to filename-based config is the desired state; first apply after import
  # will update the trigger to read from cloudbuild.yaml.
  filename = "cloudbuild.yaml"

  substitutions = {
    _CONTENT_BUCKET   = google_storage_bucket.hugo_content.name
    _FIREBASE_PROJECT = var.project_id
  }
}

# Push-to-main trigger for CI. Uses autodetect to find build config in repo.
# GitHub App connection is a prerequisite managed outside Terraform
# (Cloud Console → Cloud Build → Repositories).
resource "google_cloudbuild_trigger" "transcript_summarizer_push" {
  project  = var.project_id
  name     = "transcript-summarizer-push"
  location = "global"

  service_account = "projects/${var.project_id}/serviceAccounts/${google_service_account.transcript_summarizer.email}"

  github {
    owner = var.github_owner
    name  = var.github_repo

    push {
      branch = "^main$"
    }
  }

  # Live state uses autodetect=true (not a valid TF argument in provider v6).
  # filename = "cloudbuild.yaml" is equivalent and more explicit.
  filename = "cloudbuild.yaml"
}
