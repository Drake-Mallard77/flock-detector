# Cost guidance

The infrastructure is optimized for a low-traffic MVP. Deploy only one cloud stack.

| Component | Default choice | Cost rationale |
|---|---|---|
| Compute | One small VM | The application and PostgreSQL share one host. |
| Database | PostgreSQL container | Avoids a separate managed-database minimum charge. |
| Storage | 30–50 GB boot volume | Enough for the application and an early dataset. |
| Networking | Public subnet and public IP | Avoids a NAT gateway and load balancer. |
| Containers | GitHub Container Registry | VM can build from the public repository if the image is unavailable. |
| TLS/domain | Not initially provisioned | Add before a broad public production launch. |

## Default sizes

| Provider | Default | Notes |
|---|---|---|
| AWS | `t3.small` | 2 GB RAM; reduce to `t3.micro` only after monitoring memory. |
| Azure | `Standard_B1ms` | Burstable 2 GB VM for light workloads. |
| GCP | `e2-small` | Shared-core 2 GB VM for low traffic. |
| OCI | `VM.Standard.A1.Flex`, 1 OCPU / 6 GB | May fit Always Free allowances when capacity is available. |

Pricing varies by region, discounts, egress, storage, taxes, and free-tier eligibility. Review the chosen provider's calculator immediately before `terraform apply`.

## Upgrade triggers

- Add object storage when evidence uploads are implemented.
- Add a managed database when losing one VM would exceed the acceptable recovery window.
- Add a load balancer only when using two or more application instances.
- Add a CDN when public bandwidth becomes material.
- Add a secrets manager before handling production credentials or adding operators.

Destroy unused test environments promptly:

```bash
terraform destroy
```
