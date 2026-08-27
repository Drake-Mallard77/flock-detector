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
    # KNOWN COSMETIC DRIFT: every plan reports this block as changing
    # (min_instance_count/manual_instance_count 0 -> null). The Cloud Run
    # API returns 0 while the provider wants null, so the diff reappears
    # whether the field is set explicitly or omitted entirely — both were
    # tried. It is a no-op; scale-to-zero is unaffected. Deliberately NOT
    # silenced with ignore_changes, which would also hide real scaling
    # changes. Keep it in mind when reading a plan.
    scaling {
      min_instance_count = 0
      max_instance_count = 3
    }

    containers {
      image = var.image

      # Read by the Caddyfile, not by the bundle: Vite inlines its own env
      # at build time, but Caddy resolves {$VAR} at startup. These configure
      # routing (canonical host, where to proxy the sitemap), so they belong
      # to the deployment rather than the build.
      dynamic "env" {
        for_each = var.env
        content {
          name  = env.key
          value = env.value
        }
      }

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
