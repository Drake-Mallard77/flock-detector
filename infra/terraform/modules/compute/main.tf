locals {
  compose_content = file("${path.module}/templates/docker-compose.prod.yml")

  caddyfile_content = templatefile("${path.module}/templates/Caddyfile.tftpl", {
    domain = var.domain
  })

  # Not committed anywhere, only ever materialized inside rendered
  # user_data: OCI instance metadata (including user_data) is visible to
  # anyone with read access on the instance via the console/API, so this is
  # sensitive the same way a committed .env would be. Terraform state is the
  # only place it persists — treat state as a secret (see environments/prod
  # backend notes in docs/ARCHITECTURE.md).
  env_content = <<-EOT
    APP_IMAGE=${var.app_image}
    DB_PASSWORD=${var.db_password}
    JWT_SECRET=${var.jwt_secret}
    ALLOWED_ORIGIN=${var.allowed_origin}
  EOT

  cloud_init = templatefile("${path.module}/templates/cloud-init.yaml.tftpl", {
    compose_content    = local.compose_content
    caddyfile_content  = local.caddyfile_content
    env_content        = local.env_content
    data_volume_device = var.data_volume_device
  })
}

resource "oci_core_instance" "this" {
  compartment_id      = var.compartment_ocid
  availability_domain = var.availability_domain
  display_name        = var.name
  shape               = var.shape

  shape_config {
    ocpus         = var.ocpus
    memory_in_gbs = var.memory_gbs
  }

  create_vnic_details {
    subnet_id        = var.subnet_id
    assign_public_ip = true
    display_name     = var.name
  }

  source_details {
    source_type             = "image"
    source_id               = var.image_ocid
    boot_volume_size_in_gbs = var.boot_volume_size_gbs
  }

  metadata = {
    ssh_authorized_keys = var.ssh_public_key
    user_data           = base64encode(local.cloud_init)
  }

  # user_data only runs on first boot (cloud-init). Changing app_image,
  # db_password, jwt_secret, allowed_origin, or domain after the initial
  # apply will NOT retroactively reconfigure a running instance — Terraform
  # will show a diff on this resource, but applying it either does nothing
  # visible or (depending on what changed) forces recreation. For a config
  # change on a live instance, SSH in and update /opt/flockwatch/{.env,
  # Caddyfile} directly, then `docker compose up -d` — see
  # infra/terraform/README.md.
}
