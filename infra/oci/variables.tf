variable "tenancy_ocid" {
  type = string
}
variable "compartment_ocid" {
  type = string
}
variable "region" {
  type = string
}
variable "image_ocid" {
  type        = string
  description = "Region-specific ARM Ubuntu 24.04 image OCID for the default A1 shape."
}
variable "name" {
  type    = string
  default = "flockwatch"
}
variable "shape" {
  type    = string
  default = "VM.Standard.A1.Flex"
}
variable "ocpus" {
  type    = number
  default = 1
}
variable "memory_gbs" {
  type    = number
  default = 6
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
  type = string
}
variable "ssh_source_cidr" {
  type    = string
  default = "0.0.0.0/0"
}
