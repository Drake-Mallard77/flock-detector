variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "name" {
  type    = string
  default = "flockwatch-atlas"
}

variable "image" {
  type        = string
  description = "Fully-qualified image reference, pinned by digest. See modules/cloud-run/variables.tf for why a digest rather than a tag."
}

variable "cpu" {
  type    = string
  default = "1"
}

variable "memory" {
  type        = string
  default     = "512Mi"
  description = "Caddy serving static files needs far less than this, but Cloud Run rejects anything under 512Mi at 1 CPU. Lowering it requires lowering cpu too."
}

variable "env" {
  type        = map(string)
  default     = {}
  description = "Environment variables for the container. Read by the Caddyfile at startup; the app bundle's own config is inlined at build time instead."
}
