variable "project_id" {
  description = "GCP project ID"
  type        = string
  default     = "miltonmeetingsummarizer"
}

variable "region" {
  description = "Default GCP region for most resources"
  type        = string
  default     = "us-central1"
}

variable "project_number" {
  description = "GCP project number (used for SA email construction and source bucket name). Supply via TF_VAR_project_number — see 'make tf-apply'."
  type        = string
}

variable "github_owner" {
  description = "GitHub repository owner"
  type        = string
  default     = "kfiles"
}

variable "github_repo" {
  description = "GitHub repository name"
  type        = string
  default     = "TranscriptSummarizer"
}

variable "youtube_channel_id" {
  description = "YouTube channel ID used for PubSubHubbub subscriptions and Scheduler body"
  type        = string
  default     = "UCGnv43oWpciURP-bTDc3GnA"
}
