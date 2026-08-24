output "public_ip" {
  value = module.compute.public_ip
}

output "url" {
  value = var.domain != "" ? "https://${var.domain}" : "http://${module.compute.public_ip}"
}
