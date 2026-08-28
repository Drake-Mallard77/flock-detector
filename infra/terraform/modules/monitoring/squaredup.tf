# Read-only credential for SquaredUp (external dashboard/observability SaaS)
# to pull Cloud Run, Cloud Monitoring, and Cloud Scheduler status into its
# own dashboards. A dedicated service account, not the default compute SA:
# that SA also holds Secret Manager access (see cloud-run module), which a
# third-party credential has no reason to be able to reach.
#
# SquaredUp's GCP plugin authenticates with a service account key (no
# Workload Identity Federation support as of writing), so this only
# provisions the identity and its permissions — the key itself is
# deliberately not created here. A `google_service_account_key` resource
# would write the private key into Terraform state in plaintext. Generate it
# out-of-band instead:
#
#   gcloud iam service-accounts keys create squaredup-key.json \
#     --iam-account=${google_service_account.squaredup.email}
#
# then paste client_email, private_key, and project_id from that file into
# SquaredUp's "Add data source" form, and delete the local key file. Rotate
# by creating a new key, updating the data source, then deleting the old key
# (`gcloud iam service-accounts keys list/delete`).
resource "google_service_account" "squaredup" {
  project      = var.project_id
  account_id   = "squaredup-monitoring"
  display_name = "SquaredUp (read-only monitoring)"
}

# Scoped to what's actually deployed here — Cloud Run services/jobs, alert
# policies, and uptime checks — rather than SquaredUp's documented default
# of the broad "Viewer" basic role, which would also grant read access
# project-wide to resource types this project doesn't use.
resource "google_project_iam_member" "squaredup_monitoring_viewer" {
  project = var.project_id
  role    = "roles/monitoring.viewer"
  member  = "serviceAccount:${google_service_account.squaredup.email}"
}

resource "google_project_iam_member" "squaredup_run_viewer" {
  project = var.project_id
  role    = "roles/run.viewer"
  member  = "serviceAccount:${google_service_account.squaredup.email}"
}

resource "google_project_iam_member" "squaredup_scheduler_viewer" {
  project = var.project_id
  role    = "roles/cloudscheduler.viewer"
  member  = "serviceAccount:${google_service_account.squaredup.email}"
}

# SquaredUp's GCP plugin needs these two beyond the per-service viewer roles
# above: it enumerates the project to discover what exists, and calls the
# monitoring APIs as a consumer of this project's service usage. Without
# them the data source authenticates but returns nothing to chart.
resource "google_project_iam_member" "squaredup_browser" {
  project = var.project_id
  role    = "roles/browser"
  member  = "serviceAccount:${google_service_account.squaredup.email}"
}

resource "google_project_iam_member" "squaredup_service_usage_consumer" {
  project = var.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = "serviceAccount:${google_service_account.squaredup.email}"
}
