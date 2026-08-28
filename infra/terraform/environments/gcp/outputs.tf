output "api_url" {
  value = module.cloud_run.url
}

output "site_url" {
  description = "The public web app — this is the URL to visit and share."
  value       = module.atlas.url
}

output "squaredup_service_account_email" {
  description = "Pass to `gcloud iam service-accounts keys create --iam-account=...` when generating the key for SquaredUp's data source form."
  value       = module.monitoring.squaredup_service_account_email
}
