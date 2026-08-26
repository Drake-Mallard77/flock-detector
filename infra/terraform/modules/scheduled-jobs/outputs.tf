output "refresh_job_name" {
  value = google_cloud_run_v2_job.refresh_cameras.name
}

output "derive_job_name" {
  value = google_cloud_run_v2_job.derive_deployments.name
}
