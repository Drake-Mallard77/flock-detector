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
  default     = "256Mi"
  description = "Caddy serving static files needs very little; the app itself runs in the visitor's browser."
}
