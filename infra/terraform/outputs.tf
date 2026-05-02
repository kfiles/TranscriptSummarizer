output "function_uri" {
  description = "Cloud Run URI for the youtube-webhook function (stable, used by Scheduler)"
  value       = google_cloudfunctions2_function.youtube_webhook.service_config[0].uri
}

output "function_url" {
  description = "Cloud Functions URL (cloudfunctions.net domain)"
  value       = google_cloudfunctions2_function.youtube_webhook.url
}

output "hugo_content_bucket" {
  description = "GCS bucket name for Hugo content staging"
  value       = google_storage_bucket.hugo_content.name
}

output "pubsub_topic_id" {
  description = "Full Pub/Sub topic resource ID"
  value       = google_pubsub_topic.pipeline_trigger.id
}

output "transcript_summarizer_sa_email" {
  description = "Service account email for the transcript-summarizer function"
  value       = google_service_account.transcript_summarizer.email
}
