terraform {
  required_version = ">= 1.12.0"

  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 6.0"
    }
  }

  # Native OCI backend (Terraform >= 1.12) — Oracle's docs now recommend
  # this over the older S3-compatible-endpoint workaround for storing state
  # in Object Storage. UNVERIFIED against a real OCI account in this repo
  # (no credentials available while building this) — run `terraform init`
  # and confirm the exact required keys before trusting this in production;
  # see infra/terraform/README.md for the S3-compatible fallback if this
  # backend type doesn't behave as expected on your Terraform version.
  backend "oci" {
    # Supply via `terraform init -backend-config=backend.hcl` (gitignored)
    # or -backend-config=key=value flags, not hardcoded here:
    #   bucket    = "<your-state-bucket>"
    #   namespace = "<your-object-storage-namespace>"
    #   key       = "flockwatch/prod/terraform.tfstate"
    #   region    = "<your-region>"
  }
}

provider "oci" {
  region = var.region
}
