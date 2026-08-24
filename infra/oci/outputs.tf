output "public_ip" {
  value = oci_core_instance.this.public_ip
}
output "url" {
  value = "http://${oci_core_instance.this.public_ip}"
}
output "admin_url" {
  value = "http://${oci_core_instance.this.public_ip}/admin/"
}
