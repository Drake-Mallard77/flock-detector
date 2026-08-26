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

variable "image_tag" {
  type    = string
  default = "latest"
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
