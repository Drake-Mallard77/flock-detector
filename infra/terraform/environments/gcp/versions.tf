terraform {
  required_version = ">= 1.12.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }

  # Local state for now, same reasoning as environments/prod (OCI): get the
  # first real deploy working before adding a remote-state backend.
}

provider "google" {
  project = var.project_id
  region  = var.region
}
