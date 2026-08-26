output "url" {
  value = google_cloud_run_v2_service.api.uri
}

output "database_url_secret_id" {
  description = "Secret Manager secret holding the connection string, so other workloads reference it rather than receiving a copy."
  value       = google_secret_manager_secret.database_url.secret_id
}

output "runtime_service_account" {
  description = "Identity the API runs as; already granted secretAccessor on the secrets above."
  value       = local.default_compute_sa
}
