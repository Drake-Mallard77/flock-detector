locals {
  env = base64encode("APP_IMAGE=${var.app_image}\nDB_PASSWORD=${var.db_password}\nADMIN_USERNAME=${var.admin_username}\nADMIN_PASSWORD=${var.admin_password}\n")
  cloud_init = templatefile("${path.module}/../templates/cloud-init.yaml.tftpl", {
    compose_b64 = filebase64("${path.module}/../templates/docker-compose.yml"), env_b64 = local.env
  })
}
resource "google_compute_network" "this" {
  name                    = "${var.name}-vpc"
  auto_create_subnetworks = false
}
resource "google_compute_subnetwork" "this" {
  name          = "${var.name}-subnet"
  ip_cidr_range = "10.40.1.0/24"
  region        = var.region
  network       = google_compute_network.this.id
}
resource "google_compute_firewall" "http" {
  name    = "${var.name}-http"
  network = google_compute_network.this.name
  allow {
    protocol = "tcp"
    ports    = ["80"]
  }
  source_ranges = ["0.0.0.0/0"]
  target_tags   = [var.name]
}
resource "google_compute_firewall" "ssh" {
  name    = "${var.name}-ssh"
  network = google_compute_network.this.name
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
  source_ranges = [var.ssh_source_cidr]
  target_tags   = [var.name]
}
resource "google_compute_address" "this" {
  name   = "${var.name}-ip"
  region = var.region
}
resource "google_compute_instance" "this" {
  name         = var.name
  machine_type = var.machine_type
  zone         = var.zone
  tags         = [var.name]
  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64"
      size  = 30
      type  = "pd-balanced"
    }
  }
  network_interface {
    subnetwork = google_compute_subnetwork.this.id
    access_config {
      nat_ip = google_compute_address.this.address
    }
  }
  metadata = {
    ssh-keys = var.ssh_public_key != "" ? "ubuntu:${var.ssh_public_key}" : ""
  }
  metadata_startup_script = local.cloud_init
  service_account {
    scopes = ["logging-write", "monitoring-write"]
  }
  shielded_instance_config {
    enable_secure_boot          = true
    enable_vtpm                 = true
    enable_integrity_monitoring = true
  }
}
