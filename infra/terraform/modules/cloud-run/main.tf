data "google_project" "this" {
  project_id = var.project_id
}

# Cloud Run's default runtime service account, used when a service doesn't
# specify its own. Granted Secret Manager access below rather than creating
# a dedicated service account — this is a single low-traffic service, not a
# multi-service system where per-service SA isolation earns its complexity.
locals {
  default_compute_sa = "${data.google_project.this.number}-compute@developer.gserviceaccount.com"
}

# Mirrors ghcr.io through Artifact Registry rather than Cloud Run pulling
# directly from GHCR: Google's own docs recommend this for reliability —
# direct public-GHCR pulls are cached only up to an hour and are explicitly
# called out as less reliable for production than a proper AR mirror.
resource "google_artifact_registry_repository" "ghcr_mirror" {
  project       = var.project_id
  location      = var.region
  repository_id = "ghcr-mirror"
  format        = "DOCKER"
  mode          = "REMOTE_REPOSITORY"

  remote_repository_config {
    description = "Mirror of ghcr.io for pulling the FlockWatch API image"
    docker_repository {
      custom_repository {
        uri = "https://ghcr.io"
      }
    }
  }
}

resource "google_secret_manager_secret" "database_url" {
  project   = var.project_id
  secret_id = "${var.name}-database-url"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "database_url" {
  secret      = google_secret_manager_secret.database_url.id
  secret_data = var.database_url
}

resource "google_secret_manager_secret_iam_member" "database_url_access" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.database_url.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.default_compute_sa}"
}

resource "google_secret_manager_secret" "jwt_secret" {
  project   = var.project_id
  secret_id = "${var.name}-jwt-secret"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "jwt_secret" {
  secret      = google_secret_manager_secret.jwt_secret.id
  secret_data = var.jwt_secret
}

resource "google_secret_manager_secret_iam_member" "jwt_secret_access" {
  project   = var.project_id
  secret_id = google_secret_manager_secret.jwt_secret.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${local.default_compute_sa}"
}

resource "google_cloud_run_v2_service" "api" {
  project  = var.project_id
  name     = var.name
  location = var.region
  ingress  = "INGRESS_TRAFFIC_ALL"

  template {
    # Scale-to-zero is the whole point of choosing Cloud Run over a VM here:
    # at low/idle traffic, actual cost approaches $0 rather than paying for
    # capacity reserved 24/7. See docs/ARCHITECTURE.md.
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
      image = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.ghcr_mirror.repository_id}/${var.ghcr_owner_repo}@${var.image_digest}"

      resources {
        limits = {
          cpu    = var.cpu
          memory = var.memory
        }
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url.secret_id
            version = "latest"
          }
        }
      }

      env {
        name = "JWT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.jwt_secret.secret_id
            version = "latest"
          }
        }
      }

      env {
        name  = "ALLOWED_ORIGIN"
        value = var.allowed_origin
      }

      env {
        name  = "ENV"
        value = "production"
      }

      # Plain env var, not Secret Manager: this is a public identifier that
      # already ships in the browser bundle. Its role is to pin the expected
      # audience when validating Google ID tokens, not to authenticate us.
      env {
        name  = "GOOGLE_CLIENT_ID"
        value = var.google_client_id
      }
    }
  }

  depends_on = [
    google_secret_manager_secret_iam_member.database_url_access,
    google_secret_manager_secret_iam_member.jwt_secret_access,
  ]
}

# Public API — the moderator-only endpoints are still gated by the app's
# own JWT auth (internal/httpapi/auth.go), this just controls whether Cloud
# Run's own IAM layer requires a Google-signed request on top of that.
resource "google_cloud_run_v2_service_iam_member" "public" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.api.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
