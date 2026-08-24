variable "compartment_ocid" {
  type = string
}

variable "availability_domain" {
  type = string
}

variable "subnet_id" {
  type = string
}

variable "name" {
  type    = string
  default = "flockwatch"
}

variable "shape" {
  type        = string
  default     = "VM.Standard.A1.Flex"
  description = "Always Free ARM Ampere shape."
}

variable "ocpus" {
  type    = number
  default = 2
}

variable "memory_gbs" {
  type    = number
  default = 12
}

variable "boot_volume_size_gbs" {
  type        = number
  default     = 50
  description = "Kept modest, alongside the storage module's data volume default of 50GB, to stay well under the Always Free 200GB combined boot+block budget."
}

variable "image_ocid" {
  type        = string
  description = "Region-specific Ubuntu ARM (aarch64) image OCID for the A1.Flex shape."
}

variable "ssh_public_key" {
  type = string
}

variable "data_volume_device" {
  type        = string
  default     = "/dev/oracleoci/oraclevdb"
  description = "Must match the device path the storage module attaches the data volume at."
}

variable "app_image" {
  type        = string
  description = "Container image reference for the API, e.g. ghcr.io/<owner>/flock-detector-api:latest."
}

variable "db_password" {
  type      = string
  sensitive = true
  validation {
    condition     = length(var.db_password) >= 16
    error_message = "Use at least 16 characters."
  }
}

variable "jwt_secret" {
  type      = string
  sensitive = true
  validation {
    condition     = length(var.jwt_secret) >= 32
    error_message = "JWT_SECRET signs moderator authorization tokens — use at least 32 random characters (e.g. `openssl rand -base64 32`), not a memorable phrase."
  }
}

variable "allowed_origin" {
  type        = string
  description = "CORS origin the API will accept, e.g. https://flockwatch.example.org once the web app (Phase 4) is deployed."
}

variable "domain" {
  type        = string
  default     = ""
  description = "Public domain for Caddy's automatic HTTPS. Empty deploys over plain HTTP on the bare IP for the first reachability check; set this (and point DNS at the instance's public IP) once you have one."
}
