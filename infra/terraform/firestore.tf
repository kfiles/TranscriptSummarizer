resource "google_firestore_database" "meetingtranscripts" {
  project  = var.project_id
  name     = "meetingtranscripts"
  # Firestore chose nam5 (multi-region) at database creation time; cannot be changed.
  location_id = "nam5"
  type        = "FIRESTORE_NATIVE"

  concurrency_mode            = "PESSIMISTIC"
  app_engine_integration_mode = "DISABLED"

  delete_protection_state = "DELETE_PROTECTION_DISABLED"
}
