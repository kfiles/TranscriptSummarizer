terraform {
  required_version = ">= 1.5"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }

  # Bootstrap: create the state bucket before enabling this backend.
  #   gcloud storage buckets create gs://miltonmeetingsummarizer-tfstate \
  #     --location=us-central1 --uniform-bucket-level-access \
  #     --project=miltonmeetingsummarizer
  # Then run: terraform init -migrate-state
  backend "gcs" {
    bucket = "miltonmeetingsummarizer-tfstate"
    prefix = "terraform/state"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
