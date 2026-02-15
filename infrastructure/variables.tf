variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region for all resources"
  type        = string
  default     = "australia-southeast1"
}

variable "github_org" {
  description = "GitHub organisation or user that owns the repository"
  type        = string
}

variable "github_repo" {
  description = "GitHub repository name"
  type        = string
  default     = "weekday-masters"
}

variable "backend_service_name" {
  description = "Cloud Run service name for the backend"
  type        = string
  default     = "rally-api"
}

variable "artifact_registry_repo" {
  description = "Artifact Registry repository name"
  type        = string
  default     = "weekday-masters"
}

variable "firebase_project_id" {
  description = "Firebase project ID (usually same as GCP project ID)"
  type        = string
}

variable "firebase_site_id" {
  description = "Firebase Hosting site ID"
  type        = string
  default     = "rally-club"
}

variable "wif_pool_id" {
  description = "Workload Identity Federation pool ID"
  type        = string
  default     = "github-actions-pool"
}

variable "wif_provider_id" {
  description = "Workload Identity Federation provider ID"
  type        = string
  default     = "github-actions-provider"
}
