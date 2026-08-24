data "oci_identity_availability_domains" "this" {
  compartment_id = var.tenancy_ocid
}
locals {
  env = base64encode("APP_IMAGE=${var.app_image}\nDB_PASSWORD=${var.db_password}\nADMIN_USERNAME=${var.admin_username}\nADMIN_PASSWORD=${var.admin_password}\n")
  cloud_init = templatefile("${path.module}/../templates/cloud-init.yaml.tftpl", {
    compose_b64 = filebase64("${path.module}/../templates/docker-compose.yml"), env_b64 = local.env
  })
}
resource "oci_core_vcn" "this" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-vcn"
  cidr_blocks    = ["10.50.0.0/16"]
  dns_label      = "flockwatch"
}
resource "oci_core_internet_gateway" "this" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.this.id
  display_name   = "${var.name}-igw"
  enabled        = true
}
resource "oci_core_route_table" "public" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.this.id
  display_name   = "${var.name}-public"
  route_rules {
    network_entity_id = oci_core_internet_gateway.this.id
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
  }
}
resource "oci_core_security_list" "this" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.this.id
  display_name   = "${var.name}-security"
  egress_security_rules {
    protocol    = "all"
    destination = "0.0.0.0/0"
  }
  ingress_security_rules {
    protocol = "6"
    source   = "0.0.0.0/0"
    tcp_options {
      min = 80
      max = 80
    }
  }
  ingress_security_rules {
    protocol = "6"
    source   = var.ssh_source_cidr
    tcp_options {
      min = 22
      max = 22
    }
  }
}
resource "oci_core_subnet" "public" {
  compartment_id             = var.compartment_ocid
  vcn_id                     = oci_core_vcn.this.id
  display_name               = "${var.name}-public"
  cidr_block                 = "10.50.1.0/24"
  dns_label                  = "public"
  route_table_id             = oci_core_route_table.public.id
  security_list_ids          = [oci_core_security_list.this.id]
  prohibit_public_ip_on_vnic = false
}
resource "oci_core_instance" "this" {
  compartment_id      = var.compartment_ocid
  availability_domain = data.oci_identity_availability_domains.this.availability_domains[0].name
  display_name        = var.name
  shape               = var.shape
  shape_config {
    ocpus         = var.ocpus
    memory_in_gbs = var.memory_gbs
  }
  create_vnic_details {
    subnet_id        = oci_core_subnet.public.id
    assign_public_ip = true
    display_name     = var.name
  }
  source_details {
    source_type             = "image"
    source_id               = var.image_ocid
    boot_volume_size_in_gbs = 50
  }
  metadata = {
    ssh_authorized_keys = var.ssh_public_key
    user_data           = base64encode(local.cloud_init)
  }
}
