output "api_url" {
  value = module.cloud_run.url
}

output "site_url" {
  description = "The public web app — this is the URL to visit and share."
  value       = module.atlas.url
}
