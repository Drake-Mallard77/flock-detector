variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "ghcr_owner_repo" {
  type        = string
  default     = "drake-mallard77/flock-detector-api"
  description = "Path portion of the GHCR image, after the ghcr.io host."
}

variable "image_digest" {
  type        = string
  description = "sha256:... digest of the published image. See modules/cloud-run/variables.tf for why this is a digest, not a tag."
}

variable "atlas_ghcr_owner_repo" {
  type        = string
  default     = "drake-mallard77/flock-detector-atlas"
  description = "Path portion of the web app's GHCR image, after the ghcr.io host."
}

variable "atlas_image_digest" {
  type        = string
  description = "sha256:... digest of the published web app image. See modules/cloud-run/variables.tf for why a digest, not a tag."
}

variable "database_url" {
  type      = string
  sensitive = true
}

variable "jwt_secret" {
  type      = string
  sensitive = true
}

// allowed_origin is intentionally not a variable: it's derived from
// module.atlas.url so the API's CORS policy can't drift from where the site
// is actually served. For local development, run the API from
// infra/docker/docker-compose.yml, which sets it to localhost:5173.

variable "google_client_id" {
  type        = string
  default     = ""
  description = "OAuth client ID for Google sign-in. Public by design; see modules/cloud-run/variables.tf."
}

variable "importer_ghcr_owner_repo" {
  type        = string
  default     = "drake-mallard77/flock-detector-importer"
  description = "Path portion of the importer's GHCR image, after the ghcr.io host."
}

variable "importer_image_digest" {
  type        = string
  description = "sha256:... digest of the published importer image. See modules/cloud-run/variables.tf for why a digest, not a tag."
}

variable "site_domain" {
  type        = string
  default     = ""
  description = "Custom domain the site is served from. Empty means Cloud Run URLs only. Adding it here is what permits browser requests from that origin."
}
