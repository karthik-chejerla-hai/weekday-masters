resource "google_firebase_hosting_site" "frontend" {
  provider = google-beta
  project  = var.firebase_project_id
  site_id  = var.firebase_site_id

  depends_on = [google_project_service.apis]
}
