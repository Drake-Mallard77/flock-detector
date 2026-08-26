module "cloud_run" {
  source = "../../modules/cloud-run"

  project_id       = var.project_id
  region           = var.region
  ghcr_owner_repo  = var.ghcr_owner_repo
  image_digest     = var.image_digest
  database_url     = var.database_url
  jwt_secret       = var.jwt_secret
  google_client_id = var.google_client_id

  # Point CORS at the deployed site rather than a hardcoded value, so the
  # two can't drift apart. Terraform resolves the dependency order; a
  # mismatch here means every browser request from the site is blocked.
  allowed_origin = module.atlas.url
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
