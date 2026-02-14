# ---------- Backend deployer ----------
resource "google_service_account" "backend_deployer" {
  account_id   = "rally-backend-deployer"
  display_name = "Rally Backend Deployer (GitHub Actions)"

  depends_on = [google_project_service.apis]
}

resource "google_project_iam_member" "backend_ar_writer" {
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.backend_deployer.email}"
}

resource "google_project_iam_member" "backend_run_admin" {
  project = var.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.backend_deployer.email}"
}

resource "google_project_iam_member" "backend_sa_user" {
  project = var.project_id
  role    = "roles/iam.serviceAccountUser"
  member  = "serviceAccount:${google_service_account.backend_deployer.email}"
}

# ---------- Frontend deployer ----------
resource "google_service_account" "frontend_deployer" {
  account_id   = "rally-frontend-deployer"
  display_name = "Rally Frontend Deployer (GitHub Actions)"

  depends_on = [google_project_service.apis]
}

resource "google_project_iam_member" "frontend_firebase_hosting_admin" {
  project = var.project_id
  role    = "roles/firebasehosting.admin"
  member  = "serviceAccount:${google_service_account.frontend_deployer.email}"
}

resource "google_project_iam_member" "frontend_firebase_viewer" {
  project = var.project_id
  role    = "roles/firebase.viewer"
  member  = "serviceAccount:${google_service_account.frontend_deployer.email}"
}
