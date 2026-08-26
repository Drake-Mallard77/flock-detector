variable "project_id" {
  type = string
}

variable "region" {
  type    = string
  default = "us-central1"
}

variable "image" {
  type        = string
  description = "Importer image, pinned by digest. See modules/cloud-run/variables.tf for why a digest rather than a tag."
}

variable "database_url_secret_id" {
  type        = string
  description = "Secret Manager secret holding the Postgres connection string. Passed by reference so the credential never appears in job config or Terraform variables."
}

variable "service_account_email" {
  type        = string
  description = "Identity the jobs run as; needs secretAccessor on the database secret."
}

variable "refresh_schedule" {
  type        = string
  default     = "0 4 * * 1"
  description = <<-EOT
    Weekly, Monday 04:00. OSM edits don't move fast enough to justify more,
    and a full run makes 51 Overpass queries over ~20 minutes against a
    free shared service whose usage policy asks clients not to hammer it.
  EOT
}

variable "derive_schedule" {
  type        = string
  default     = "0 6 * * 1"
  description = "Two hours after the refresh, so candidates are derived from freshly imported operator tags rather than the previous week's."
}

variable "time_zone" {
  type    = string
  default = "Etc/UTC"
}
