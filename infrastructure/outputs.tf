output "wif_provider" {
  description = "Workload Identity Federation provider resource name (for GitHub Actions)"
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "backend_deployer_sa_email" {
  description = "Backend deployer service account email"
  value       = google_service_account.backend_deployer.email
}

output "frontend_deployer_sa_email" {
  description = "Frontend deployer service account email"
  value       = google_service_account.frontend_deployer.email
}

output "artifact_registry_url" {
  description = "Artifact Registry Docker repository URL"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.docker.repository_id}"
}

output "cloud_run_url" {
  description = "Cloud Run service URL"
  value       = google_cloud_run_v2_service.backend.uri
}

output "firebase_site_url" {
  description = "Firebase Hosting URL"
  value       = "https://${var.firebase_site_id}.web.app"
}

output "github_secrets_summary" {
  description = "GitHub Secrets to configure (copy-paste friendly)"
  value       = <<-EOT

    ┌─────────────────────────────────────────────────────┐
    │  GitHub Secrets to set                              │
    ├──────────────────────┬──────────────────────────────┤
    │  WIF_PROVIDER        │  ${google_iam_workload_identity_pool_provider.github.name}
    │  BACKEND_SA_EMAIL    │  ${google_service_account.backend_deployer.email}
    │  FRONTEND_SA_EMAIL   │  ${google_service_account.frontend_deployer.email}
    └──────────────────────┴──────────────────────────────┘

    After verifying deploys work, delete the old GCP_SA_KEY secret.
  EOT
}
