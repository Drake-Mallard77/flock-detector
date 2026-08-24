variable "compartment_ocid" {
  type = string
}

variable "name" {
  type    = string
  default = "flockwatch"
}

variable "vcn_cidr" {
  type    = string
  default = "10.50.0.0/16"
}

variable "subnet_cidr" {
  type    = string
  default = "10.50.1.0/24"
}

variable "ssh_source_cidr" {
  type        = string
  description = "CIDR allowed to reach port 22. Set this to your own IP/32, not 0.0.0.0/0 — this is a public-records site that will draw hostile attention, and open SSH is the single most common way small VMs get compromised."
}

variable "http_source_cidrs" {
  type        = list(string)
  description = <<-EOT
    CIDRs allowed to reach ports 80/443. Defaults to the whole internet
    because nothing fronts the origin yet. Once Cloudflare is in front of
    this (planned — see docs/ARCHITECTURE.md), narrow this to Cloudflare's
    published IP ranges (https://www.cloudflare.com/ips/) so the origin
    only accepts traffic that came through Cloudflare's WAF/DDoS layer,
    and an attacker who finds the origin IP directly gets nothing.
  EOT
  default     = ["0.0.0.0/0"]
}
