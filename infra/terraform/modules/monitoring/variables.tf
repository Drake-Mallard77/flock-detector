variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "alert_email" {
  type        = string
  description = "Where alerts go. Without this, a failed weekly import is silent and the data quietly goes stale — the failure mode this module exists to prevent."
}

variable "api_url" {
  type        = string
  description = "Public API base URL, probed for uptime."
}

variable "site_url" {
  type        = string
  description = "Public site URL, probed for uptime."
}
