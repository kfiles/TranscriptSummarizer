resource "google_storage_bucket" "hugo_content" {
  project  = var.project_id
  name     = "${var.project_id}-hugo-content"
  location = upper(var.region)

  storage_class               = "STANDARD"
  uniform_bucket_level_access = true

  soft_delete_policy {
    retention_duration_seconds = 604800
  }
}
