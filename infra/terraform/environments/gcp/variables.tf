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
