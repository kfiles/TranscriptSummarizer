resource "google_pubsub_topic" "pipeline_trigger" {
  project = var.project_id
  name    = "youtube-pipeline-trigger"
}
