variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "name" {
  type    = string
  default = "flockwatch-api"
}

variable "ghcr_owner_repo" {
  type        = string
  description = "GHCR path after the registry host, e.g. 'drake-mallard77/flock-detector-api'. Combined with the Artifact Registry remote-repository mirror to build the actual pulled image path."
}

variable "image_digest" {
  type        = string
  description = <<-EOT
    sha256:... digest of the published image, not a floating tag. Deploying
    by tag (e.g. :latest) doesn't reliably pick up new pushes: Terraform
    sees the same image string on every apply and skips the update, and
    even a forced `gcloud run deploy` with the same tag can serve a stale
    pull from the Artifact Registry remote-repository mirror's own tag
    cache. Get the current digest with:
      docker pull ghcr.io/<owner>/<repo>:latest  (prints the digest)
  EOT
}

variable "database_url" {
  type        = string
  sensitive   = true
  description = "Full Postgres connection string (Neon). Stored in Secret Manager, not as a plain env var."
}

variable "jwt_secret" {
  type      = string
  sensitive = true
}

variable "allowed_origin" {
  type    = string
  default = "http://localhost:5173"
}

variable "cpu" {
  type    = string
  default = "1"
}

variable "memory" {
  type    = string
  default = "512Mi"
}
