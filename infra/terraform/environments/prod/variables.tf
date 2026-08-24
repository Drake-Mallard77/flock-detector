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
  description = "Region-specific Ubuntu ARM (aarch64) image OCID for VM.Standard.A1.Flex. Look this up per-region: OCI Console > Compute > Images, or `oci compute image list`."
}

variable "ssh_public_key" {
  type = string
}

variable "ssh_source_cidr" {
  type        = string
  description = "Your own IP/32. See modules/network/variables.tf for why this must not be 0.0.0.0/0."
}

variable "http_source_cidrs" {
  type    = list(string)
  default = ["0.0.0.0/0"]
}

variable "app_image" {
  type        = string
  description = "e.g. ghcr.io/<owner>/flock-detector-api:latest"
}

variable "db_password" {
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

variable "domain" {
  type    = string
  default = ""
}
