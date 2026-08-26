terraform {
  required_version = ">= 1.12.0"

  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 6.0"
    }
  }

  # Remote state (native `oci` backend, Terraform >= 1.12) intentionally
  # deferred: it's unverified against a real account, and the first real
  # deploy isn't the moment to debug an unverified backend. Using local
  # state (the default when no `backend` block is present) for now — the
  # actual state file is infra/terraform/environments/prod/terraform.tfstate
  # (gitignored; back it up yourself until remote state is set up).
  # To switch on remote state later:
  #   1. Uncomment this block and fill in bucket/namespace/key/region
  #      (via -backend-config, not hardcoded here — see README).
  #   2. Run `terraform init -migrate-state`.
  # backend "oci" {
  # }
}

provider "oci" {
  region = var.region
}
