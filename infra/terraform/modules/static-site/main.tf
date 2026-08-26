# Serves the built SPA from Caddy on Cloud Run. Same scale-to-zero
# reasoning as the API: no idle cost, no capacity to reserve. Deliberately
# holds no secrets and talks to no database — everything it needs comes from
# the public API at runtime.
resource "google_cloud_run_v2_service" "site" {
  project  = var.project_id
  name     = var.name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }

    containers {
      image = var.image

      resources {
        limits = {
          cpu    = var.cpu
          memory = var.memory
        }
      }
    }
  }
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.site.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
