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

variable "database_url" {
  type      = string
  sensitive = true
}

variable "jwt_secret" {
  type      = string
  sensitive = true
}

variable "allowed_origin" {
  type    = string
  default = "http://localhost:5173"
}
