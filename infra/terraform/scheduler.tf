resource "google_cloud_scheduler_job" "renew_youtube_subscription" {
  project  = var.project_id
  name     = "renew-youtube-subscription"
  region   = var.region

  # YouTube PubSubHubbub subscriptions expire after ~10 days; renew every 9 days for buffer.
  schedule  = "0 9 */9 * *"
  time_zone = "Etc/UTC"

  attempt_deadline = "180s"

  retry_config {
    max_backoff_duration = "3600s"
    max_doublings        = 5
    max_retry_duration   = "0s"
    min_backoff_duration = "5s"
  }

  http_target {
    uri         = "https://pubsubhubbub.appspot.com/subscribe"
    http_method = "POST"

    # Body references the function's Cloud Run URI (stable across revisions).
    body = base64encode(join("&", [
      "hub.callback=${google_cloudfunctions2_function.youtube_webhook.service_config[0].uri}",
      "hub.topic=https://www.youtube.com/xml/feeds/videos.xml?channel_id=${var.youtube_channel_id}",
      "hub.mode=subscribe",
      "hub.verify=async",
    ]))

    headers = {
      "Content-Type" = "application/x-www-form-urlencoded"
    }
  }
}
