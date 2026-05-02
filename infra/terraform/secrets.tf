locals {
  all_secrets = toset([
    "facebook-page-id",
    "facebook-page-token",
    "firebase-ci-token",
    "firebase-sa-key",
    "mongodb-uri",
    "openai-api-key",
    "supadata-api-key",
    "transcriptapi-api-key",
    "youtube-api-key",
  ])

  # Secrets the function SA needs at runtime (injected via --set-secrets)
  function_secrets = toset([
    "facebook-page-id",
    "facebook-page-token",
    "mongodb-uri",
    "openai-api-key",
    "supadata-api-key",
    "transcriptapi-api-key",
    "youtube-api-key",
  ])
}

resource "google_secret_manager_secret" "secrets" {
  for_each  = local.all_secrets
  project   = var.project_id
  secret_id = each.key

  replication {
    auto {}
  }
}

# Grant transcript-summarizer read access to each runtime secret.
# Values are injected at Cloud Function deploy time, not pulled at runtime.
resource "google_secret_manager_secret_iam_member" "function_accessor" {
  for_each = local.function_secrets

  project   = var.project_id
  secret_id = google_secret_manager_secret.secrets[each.key].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.ts_sa_member
}

# firebase-ci-token and firebase-sa-key are also accessible by transcript-summarizer
# (not injected into function, used in other contexts)
resource "google_secret_manager_secret_iam_member" "ts_firebase_ci_token" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.secrets["firebase-ci-token"].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.ts_sa_member
}

resource "google_secret_manager_secret_iam_member" "ts_firebase_sa_key" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.secrets["firebase-sa-key"].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = local.ts_sa_member
}
