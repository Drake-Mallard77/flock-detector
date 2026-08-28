output "notification_channel" {
  value = google_monitoring_notification_channel.email.id
}

output "squaredup_service_account_email" {
  value       = google_service_account.squaredup.email
  description = "Pass to `gcloud iam service-accounts keys create --iam-account=...` when generating the key for SquaredUp's data source form."
}
