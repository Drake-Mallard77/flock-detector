# A dedicated data volume, separate from the boot volume: Postgres's data
# lives here so the instance can be destroyed/recreated (OS upgrade, shape
# change, region move) without losing the database. See
# docs/ARCHITECTURE.md's rollout notes on why this isn't just "Postgres on
# the boot volume" like a quick prototype might do.
resource "oci_core_volume" "data" {
  compartment_id      = var.compartment_ocid
  availability_domain = var.availability_domain
  display_name        = "${var.name}-data"
  size_in_gbs         = var.volume_size_gbs
}

# paravirtualized (not iscsi): the device just appears, no iscsiadm login
# dance needed in cloud-init. /dev/oracleoci/oraclevdb is the predictable
# udev alias OCI's Ubuntu images provide via oci-utils, used by cloud-init
# to mount it at a fixed path regardless of underlying /dev/sdX ordering.
resource "oci_core_volume_attachment" "data" {
  attachment_type = "paravirtualized"
  instance_id     = var.instance_id
  volume_id       = oci_core_volume.data.id
  device          = "/dev/oracleoci/oraclevdb"
}

resource "oci_core_volume_backup_policy" "daily" {
  compartment_id = var.compartment_ocid
  display_name   = "${var.name}-daily-backup"

  schedules {
    backup_type       = "INCREMENTAL"
    period            = var.backup_schedule
    retention_seconds = 7 * 24 * 60 * 60 # 7 days
    time_zone         = "UTC"
  }
}

resource "oci_core_volume_backup_policy_assignment" "data" {
  asset_id  = oci_core_volume.data.id
  policy_id = oci_core_volume_backup_policy.daily.id
}
