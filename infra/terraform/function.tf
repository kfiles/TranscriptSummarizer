resource "google_cloudfunctions2_function" "youtube_webhook" {
  project  = var.project_id
  name     = "youtube-webhook"
  location = var.region

  build_config {
    runtime     = "go124"
    entry_point = "YouTubeWebhook"

    # Artifact Registry repo created and managed by Cloud Functions infrastructure.
    docker_repository = "projects/${var.project_id}/locations/${var.region}/repositories/gcf-artifacts"

    # Build SA defaults to the compute default SA (108319055995-compute@developer.gserviceaccount.com).
    # Cloud Functions infrastructure manages this; do not change without testing.
    service_account = "projects/${var.project_id}/serviceAccounts/${var.project_number}-compute@developer.gserviceaccount.com"

    # Enables automatic base-image security updates.
    automatic_update_policy {}

    source {
      storage_source {
        # Cloud Functions uploads source here on each `gcloud functions deploy` / `make deploy`.
        # Terraform ignores this block (see lifecycle below); source updates happen via make deploy.
        bucket = "gcf-v2-sources-${var.project_number}-${var.region}"
        object = "youtube-webhook/function-source.zip"
      }
    }
  }

  service_config {
    service_account_email = google_service_account.transcript_summarizer.email

    available_memory                 = "512Mi"
    available_cpu                    = "0.3333"
    timeout_seconds                  = 540
    max_instance_count               = 60
    max_instance_request_concurrency = 1
    ingress_settings                 = "ALLOW_ALL"
    all_traffic_on_latest_revision   = true

    environment_variables = {
      FACEBOOK_ENABLED    = "true"
      GCS_BUCKET          = google_storage_bucket.hugo_content.name
      HUGO_CONTENT_DIR    = "/tmp/hugo-content/minutes"
      PUBSUB_PROJECT      = var.project_id
      PUBSUB_TOPIC        = google_pubsub_topic.pipeline_trigger.name
      TRANSCRIPT_PROVIDER = "transcriptapi"
    }

    secret_environment_variables {
      key        = "CHATGPT_API_KEY"
      project_id = var.project_number
      secret     = "openai-api-key"
      version    = "latest"
    }

    secret_environment_variables {
      key        = "FACEBOOK_PAGE_ID"
      project_id = var.project_number
      secret     = "facebook-page-id"
      version    = "latest"
    }

    secret_environment_variables {
      key        = "FACEBOOK_PAGE_TOKEN"
      project_id = var.project_number
      secret     = "facebook-page-token"
      version    = "latest"
    }

    secret_environment_variables {
      key        = "MONGODB_URI"
      project_id = var.project_number
      secret     = "mongodb-uri"
      version    = "latest"
    }

    secret_environment_variables {
      key        = "SUPADATA_API_KEY"
      project_id = var.project_number
      secret     = "supadata-api-key"
      version    = "latest"
    }

    secret_environment_variables {
      key        = "TRANSCRIPTAPI_API_KEY"
      project_id = var.project_number
      secret     = "transcriptapi-api-key"
      version    = "latest"
    }

    secret_environment_variables {
      key        = "YOUTUBE_API_KEY"
      project_id = var.project_number
      secret     = "youtube-api-key"
      version    = "latest"
    }
  }

  lifecycle {
    # Source updates are managed by `make deploy` (gcloud CLI), not Terraform.
    # Terraform manages service_config (env vars, secrets, scaling) and build settings
    # other than source. LOG_EXECUTION_ID is injected by the platform automatically.
    ignore_changes = [
      build_config[0].source,
      labels,
    ]
  }

  depends_on = [
    google_project_service.apis,
    google_service_account.transcript_summarizer,
    google_secret_manager_secret_iam_member.function_accessor,
  ]
}
