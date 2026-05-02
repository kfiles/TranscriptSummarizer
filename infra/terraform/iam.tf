resource "google_service_account" "transcript_summarizer" {
  project      = var.project_id
  account_id   = "transcript-summarizer"
  display_name = "Transcript Summarizer Function"
}

locals {
  ts_sa_member    = "serviceAccount:${google_service_account.transcript_summarizer.email}"
  cloudbuild_sa   = "${var.project_number}@cloudbuild.gserviceaccount.com"
  cb_sa_member    = "serviceAccount:${local.cloudbuild_sa}"
}

# ── transcript-summarizer project-level bindings ──────────────────────────────

resource "google_project_iam_member" "ts_storage_object_creator" {
  project = var.project_id
  role    = "roles/storage.objectCreator"
  member  = local.ts_sa_member
}

# Live state has storage.admin in addition to objectCreator.
# Review: objectCreator is sufficient; consider removing storage.admin.
resource "google_project_iam_member" "ts_storage_admin" {
  project = var.project_id
  role    = "roles/storage.admin"
  member  = local.ts_sa_member
}

resource "google_project_iam_member" "ts_pubsub_publisher" {
  project = var.project_id
  role    = "roles/pubsub.publisher"
  member  = local.ts_sa_member
}

resource "google_project_iam_member" "ts_logging_log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = local.ts_sa_member
}

resource "google_project_iam_member" "ts_firebase_admin" {
  project = var.project_id
  role    = "roles/firebase.admin"
  member  = local.ts_sa_member
}

# Unconditional binding. setup.sh intended a conditional binding, but in live state
# the transcript-summarizer SA has datastore.user without a condition. The conditional
# datastore.user binding in the policy belongs to a Firebase-managed Firestore principal
# (principal://firestore.googleapis.com/...), not to this service account.
resource "google_project_iam_member" "ts_datastore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = local.ts_sa_member
}

# ── Cloud Build default SA project-level bindings ─────────────────────────────

resource "google_project_iam_member" "cb_firebase_hosting_admin" {
  project = var.project_id
  role    = "roles/firebasehosting.admin"
  member  = local.cb_sa_member
}

# Cloud Build SA has project-level secretmanager.secretAccessor in live state.
# Setup.sh only granted per-secret bindings, but broader project-level binding was
# added later. Manage at project level here; per-secret bindings in secrets.tf
# for transcript-summarizer only.
resource "google_project_iam_member" "cb_secret_accessor" {
  project = var.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = local.cb_sa_member
}
