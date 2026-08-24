output "volume_id" {
  value = oci_core_volume.data.id
}

output "device_path" {
  value       = oci_core_volume_attachment.data.device
  description = "Predictable device path cloud-init mounts as the Postgres data directory."
}
