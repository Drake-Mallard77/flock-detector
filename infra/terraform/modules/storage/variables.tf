variable "compartment_ocid" {
  type = string
}

variable "availability_domain" {
  type = string
}

variable "instance_id" {
  type        = string
  description = "Instance to attach the persistent data volume to."
}

variable "name" {
  type    = string
  default = "flockwatch"
}

variable "volume_size_gbs" {
  type        = number
  default     = 50
  description = "Postgres data volume size. OCI Always Free includes 200GB combined boot+block storage across the tenancy — leave headroom for the boot volume too."
}

variable "backup_schedule" {
  type        = string
  default     = "ONE_DAY"
  description = "ONE_DAY, ONE_WEEK, or ONE_MONTH. Always Free includes 5 volume backups/month, which comfortably covers a daily schedule with room for manual pre-migration snapshots."
}
