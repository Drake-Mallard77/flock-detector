output "public_ip" {
  value = google_compute_address.this.address
}
output "url" {
  value = "http://${google_compute_address.this.address}"
}
output "admin_url" {
  value = "http://${google_compute_address.this.address}/admin/"
}
