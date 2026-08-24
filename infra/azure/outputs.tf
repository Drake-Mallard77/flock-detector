output "public_ip" {
  value = azurerm_public_ip.this.ip_address
}
output "url" {
  value = "http://${azurerm_public_ip.this.ip_address}"
}
output "admin_url" {
  value = "http://${azurerm_public_ip.this.ip_address}/admin/"
}
