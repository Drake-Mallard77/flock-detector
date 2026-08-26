# Scheduled data maintenance, as Cloud Run Jobs rather than a GitHub Actions
# cron. The deciding factor is the database credential: a GitHub-hosted
# schedule would need DATABASE_URL as a repo secret, copying a production
# credential outside GCP for a job that only ever talks to GCP. Here it
# stays in Secret Manager and is injected at runtime.

locals {
  # Generous but bounded. A full US refresh runs ~20 minutes; the ceiling is
  # there to stop a hung Overpass connection billing indefinitely, not to
  # cut off normal work.
  job_timeout = "3600s"
}

resource "google_cloud_run_v2_job" "refresh_cameras" {
  project  = var.project_id
  name     = "flockwatch-refresh-cameras"
  location = var.region

  template {
    template {
      service_account = var.service_account_email
      timeout         = local.job_timeout
      # One retry: transient Overpass failures are common enough to be worth
      # retrying, but the importer is idempotent per-state and already
      # retries internally, so more would mostly re-do finished work.
      max_retries = 1

      containers {
        image   = var.image
        command = ["importer"]

        env {
          name = "DATABASE_URL"
          value_source {
            secret_key_ref {
              secret  = var.database_url_secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }
}

# Proposes agency-level records from OSM operator tags. Everything it
# creates is under_review and invisible publicly until a moderator acts —
# see services/importer/internal/derive.
resource "google_cloud_run_v2_job" "derive_deployments" {
  project  = var.project_id
  name     = "flockwatch-derive-deployments"
  location = var.region

  template {
    template {
      service_account = var.service_account_email
      timeout         = local.job_timeout
      max_retries     = 1

      containers {
        image   = var.image
        command = ["derive-deployments"]

        env {
          name = "DATABASE_URL"
          value_source {
            secret_key_ref {
              secret  = var.database_url_secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }
}

# Cloud Scheduler triggers the jobs through the Cloud Run Admin API, using
# the same service account, so no additional credential exists to leak.
resource "google_cloud_scheduler_job" "refresh_cameras" {
  project     = var.project_id
  region      = var.region
  name        = "flockwatch-refresh-cameras"
  description = "Weekly re-import of ALPR camera locations from OpenStreetMap"
  schedule    = var.refresh_schedule
  time_zone   = var.time_zone

  # A missed run is picked up by the next week's; retrying a 20-minute
  # import aggressively would be worse for Overpass than skipping one.
  attempt_deadline = "320s"

  http_target {
    http_method = "POST"
    uri = format(
      "https://%s-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/%s/jobs/%s:run",
      var.region, var.project_id, google_cloud_run_v2_job.refresh_cameras.name,
    )

    oauth_token {
      service_account_email = var.service_account_email
    }
  }
}

resource "google_cloud_scheduler_job" "derive_deployments" {
  project     = var.project_id
  region      = var.region
  name        = "flockwatch-derive-deployments"
  description = "Weekly derivation of deployment candidates for moderator review"
  schedule    = var.derive_schedule
  time_zone   = var.time_zone

  attempt_deadline = "320s"

  http_target {
    http_method = "POST"
    uri = format(
      "https://%s-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/%s/jobs/%s:run",
      var.region, var.project_id, google_cloud_run_v2_job.derive_deployments.name,
    )

    oauth_token {
      service_account_email = var.service_account_email
    }
  }
}
