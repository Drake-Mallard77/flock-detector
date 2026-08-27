module "cloud_run" {
  source = "../../modules/cloud-run"

  project_id       = var.project_id
  region           = var.region
  ghcr_owner_repo  = var.ghcr_owner_repo
  image_digest     = var.image_digest
  database_url     = var.database_url
  jwt_secret       = var.jwt_secret
  google_client_id = var.google_client_id

  # Every origin the site is reachable from. The custom domain and its www
  # form serve the same app, and the Cloud Run URL stays valid because the
  # domain mapping proxies to it — omitting any of them silently blocks all
  # data requests from that origin with no server-side error.
  allowed_origin = join(",", compact([
    module.atlas.url,
    var.site_domain != "" ? "https://${var.site_domain}" : "",
    var.site_domain != "" ? "https://www.${var.site_domain}" : "",
  ]))
}

module "atlas" {
  source = "../../modules/static-site"

  project_id = var.project_id
  region     = var.region
  image = format(
    "%s-docker.pkg.dev/%s/ghcr-mirror/%s@%s",
    var.region,
    var.project_id,
    var.atlas_ghcr_owner_repo,
    var.atlas_image_digest,
  )
}

# Scheduled data maintenance. Reuses the API's runtime service account and
# its Secret Manager reference rather than minting a second credential —
# the jobs need exactly the database access the API already has.
module "jobs" {
  source = "../../modules/scheduled-jobs"

  project_id = var.project_id
  region     = var.region
  image = format(
    "%s-docker.pkg.dev/%s/ghcr-mirror/%s@%s",
    var.region,
    var.project_id,
    var.importer_ghcr_owner_repo,
    var.importer_image_digest,
  )

  database_url_secret_id = module.cloud_run.database_url_secret_id
  service_account_email  = module.cloud_run.runtime_service_account
}

# Alerting and uptime checks. Every failure mode this project has is quiet:
# a failed weekly import keeps serving last week's data, and a broken
# endpoint returns a clean 200 to the logs while failing in the browser.
module "monitoring" {
  source = "../../modules/monitoring"

  project_id  = var.project_id
  region      = var.region
  alert_email = var.alert_email

  site_url = var.site_domain != "" ? "https://${var.site_domain}" : module.atlas.url
  api_url  = var.site_domain != "" ? "https://api.${var.site_domain}" : module.cloud_run.url
}
