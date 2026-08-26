terraform {
  required_version = ">= 1.12.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }

  # Remote state in GCS. Local state lived on one machine, so losing that
  # machine meant Terraform losing track of the running infrastructure —
  # it would try to recreate resources that already exist rather than
  # manage them. The bucket has object versioning enabled so a corrupted
  # or truncated state can be rolled back.
  #
  # State contains db_password and jwt_secret in plaintext, so treat this
  # bucket as sensitive: it is deliberately not public, and access is
  # governed by the project's IAM rather than a separate credential.
  backend "gcs" {
    bucket = "flockwatch-prod-tfstate"
    prefix = "gcp"
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}
