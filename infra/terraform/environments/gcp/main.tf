module "cloud_run" {
  source = "../../modules/cloud-run"

  project_id      = var.project_id
  region          = var.region
  ghcr_owner_repo = var.ghcr_owner_repo
  image_digest    = var.image_digest
  database_url    = var.database_url
  jwt_secret      = var.jwt_secret
  allowed_origin  = var.allowed_origin
}
