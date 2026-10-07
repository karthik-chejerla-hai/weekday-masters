resource "google_artifact_registry_repository" "docker" {
  repository_id          = var.artifact_registry_repo
  location               = var.region
  format                 = "DOCKER"
  description            = "Docker images for Rally backend"
  cleanup_policy_dry_run = false

  cleanup_policies {
    id     = "delete-old-images"
    action = "DELETE"
    condition {
      tag_state  = "ANY"
      older_than = "86400s" # 1 day
    }
  }

  cleanup_policies {
    id     = "keep-recent-versions"
    action = "KEEP"
    most_recent_versions {
      keep_count = 5
    }
  }

  depends_on = [google_project_service.apis]
}
