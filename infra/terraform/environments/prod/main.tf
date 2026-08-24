data "oci_identity_availability_domains" "this" {
  compartment_id = var.tenancy_ocid
}

locals {
  # Always Free A1.Flex capacity is scarce in some ADs/regions (see
  # docs/ARCHITECTURE.md's "known risk" note). This picks the first AD;
  # if `terraform apply` fails with an out-of-capacity error, try
  # overriding via a different index here, or a different region entirely.
  availability_domain = data.oci_identity_availability_domains.this.availability_domains[0].name
}

module "network" {
  source = "../../modules/network"

  compartment_ocid  = var.compartment_ocid
  ssh_source_cidr   = var.ssh_source_cidr
  http_source_cidrs = var.http_source_cidrs
}

module "compute" {
  source = "../../modules/compute"

  compartment_ocid    = var.compartment_ocid
  availability_domain = local.availability_domain
  subnet_id           = module.network.subnet_id
  image_ocid          = var.image_ocid
  ssh_public_key      = var.ssh_public_key
  app_image           = var.app_image
  db_password         = var.db_password
  jwt_secret          = var.jwt_secret
  allowed_origin      = var.allowed_origin
  domain              = var.domain
}

module "storage" {
  source = "../../modules/storage"

  compartment_ocid    = var.compartment_ocid
  availability_domain = local.availability_domain
  instance_id         = module.compute.instance_id
}
