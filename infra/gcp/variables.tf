variable "project_id" {
  type = string
}
variable "region" {
  type    = string
  default = "us-east1"
}
variable "zone" {
  type    = string
  default = "us-east1-b"
}
variable "name" {
  type    = string
  default = "flockwatch"
}
variable "machine_type" {
  type    = string
  default = "e2-small"
}
variable "app_image" {
  type    = string
  default = "ghcr.io/drake-mallard77/flock-detector:latest"
}
variable "admin_username" {
  type    = string
  default = "admin"
}
variable "admin_password" {
  type      = string
  sensitive = true
  validation {
    condition     = length(var.admin_password) >= 16
    error_message = "Use at least 16 characters."
  }
}
variable "db_password" {
  type      = string
  sensitive = true
  validation {
    condition     = length(var.db_password) >= 16
    error_message = "Use at least 16 characters."
  }
}
variable "ssh_public_key" {
  type    = string
  default = ""
}
variable "ssh_source_cidr" {
  type    = string
  default = "0.0.0.0/0"
}
