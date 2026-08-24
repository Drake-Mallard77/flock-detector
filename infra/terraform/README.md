# infra/terraform

Deploys FlockWatch's API + Postgres/PostGIS + Caddy to a single OCI Always
Free ARM VM. See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) for the
overall design and rationale; this file is the operational how-to.

## Layout

```
modules/
  network/   VCN, subnet, internet gateway, security list
  compute/   The instance + its cloud-init bundle (compose file, Caddyfile)
  storage/   Persistent data volume + daily backup policy
environments/
  prod/      Root module wiring the three together
```

## One-time prerequisites

1. **OCI CLI auth**: run `oci setup config` and follow the wizard — it
   generates an API signing key pair locally and writes `~/.oci/config`,
   which both the `oci` CLI and Terraform's `oracle/oci` provider read
   automatically. Then add the **public** key (never the private one) to
   your OCI user: **Profile icon → User settings → API Keys → Add API Key →
   Paste Public Key**. Verify with a harmless read-only call:
   ```
   oci iam region list
   ```
2. **An SSH key pair** for the instance: `ssh-keygen -t ed25519`.
3. **Your compartment OCID** — the tenancy OCID doubles as the root
   compartment OCID if you're deploying there (fine for a project this
   size); otherwise create/use a dedicated compartment.
4. **The region-specific Ubuntu ARM image OCID** — these change over time
   as Canonical publishes new images, so look up current ones rather than
   trusting a stale value:
   ```
   oci compute image list --compartment-id <tenancy_ocid> \
     --operating-system "Canonical Ubuntu" --shape "VM.Standard.A1.Flex" \
     --region <your-region>
   ```
5. **A published API image** — `app_image` needs to point at something
   real. `.github/workflows/publish-api-image.yml` builds and pushes
   `services/api` to `ghcr.io/<owner>/flock-detector-api` on every push to
   `main` that touches `services/api/**`; it should already exist by the
   time you're reading this, but confirm at
   `https://github.com/<owner>/flock-detector/pkgs/container/flock-detector-api`.
6. **Secrets**: `openssl rand -base64 24` for `db_password`,
   `openssl rand -base64 32` for `jwt_secret`.

## Deploying

```
cd infra/terraform/environments/prod
cp terraform.tfvars.example terraform.tfvars   # fill in your real values
terraform init
terraform plan
terraform apply
```

**Never paste an OCID, key, or secret from this process into a chat with an
AI assistant, this repo, or anywhere else public** — the only place
`terraform.tfvars` should exist is your local machine (it's gitignored) and,
if you set up remote state, inside Terraform state (also treat that as a
secret — see below).

### Remote state backend — unverified, check before trusting

`versions.tf` declares `backend "oci" {}`, Oracle's newer native backend
type (Terraform ≥ 1.12) for storing state in Object Storage. **This was
written without access to a real OCI account to test against — verify it
actually works on `terraform init` before relying on it.** If it doesn't
behave as documented on your Terraform version, the fallback is the older,
more battle-tested approach: an Object Storage bucket's S3-compatible
endpoint (`https://<namespace>.compat.objectstorage.<region>.oraclecloud.com`)
with the generic `s3` backend and OCI Customer Secret Keys. Either way,
**Terraform state contains your secrets in plaintext** (db_password,
jwt_secret) — treat the state file/bucket with the same care as the
`.tfvars` file itself.

### Known risk: Always Free capacity

`VM.Standard.A1.Flex` Always Free capacity is genuinely scarce in some
regions/availability domains. If `apply` fails with an out-of-capacity
error, try again (capacity frees up), or edit
`environments/prod/main.tf`'s `local.availability_domain` to try a
different AD index, or reduce `ocpus`/`memory_gbs` in `terraform.tfvars`
(the module defaults to 2 OCPU/12GB, the full current Always Free ARM
allocation — dropping to 1 OCPU/6GB, like the reference config this was
partly informed by, doubles your odds of finding capacity at the cost of
some headroom).

## After `apply`

- `terraform output url` gives you the reachable address — plain HTTP on
  the bare IP until you set `domain`.
- `curl $(terraform output -raw url)/healthz` should return `{"status":"ok"}`
  within a few minutes of `apply` completing (cloud-init needs time to
  install Docker, mount the data volume, and pull images).
- If it's not up after ~10 minutes, SSH in
  (`ssh ubuntu@$(terraform output -raw public_ip)`) and check
  `sudo cat /var/log/cloud-init-output.log` for what went wrong.

## Day-2 config changes (important limitation)

`user_data` (cloud-init) only runs on an instance's **first boot**.
Changing `app_image`, `db_password`, `jwt_secret`, `allowed_origin`, or
`domain` in `terraform.tfvars` and re-applying will show a Terraform diff,
but won't retroactively reconfigure a running instance. For now, either:

- **Small config change**: SSH in, edit `/opt/flockwatch/.env` or
  `/opt/flockwatch/Caddyfile` directly, then
  `cd /opt/flockwatch && sudo docker compose up -d`.
- **New app image**: SSH in and
  `cd /opt/flockwatch && sudo docker compose pull api && sudo docker compose up -d`
  — doesn't need a Terraform apply at all.
- **Anything bigger**: `terraform taint module.compute.oci_core_instance.this`
  then `apply`, which recreates the instance with fresh cloud-init. The data
  volume (module.storage) is untouched by this, so Postgres data survives —
  that's the whole reason it's a separate volume.

## Destroying

```
terraform destroy
```

Note the data volume's daily backups (5/month included in Always Free) are
**not** automatically deleted by `destroy` unless they're tied to the
volume's lifecycle — check the OCI console under Block Storage → Backups
if you want to fully clean up a test environment.

## Cost notes

Everything here defaults to Always Free shapes/sizes (`VM.Standard.A1.Flex`
at 2 OCPU/12GB, a 50GB boot volume, a 50GB data volume — 100GB combined,
under the 200GB Always Free boot+block budget). Nothing here provisions a
load balancer, NAT gateway, or managed database, all of which have real
costs. Upgrade triggers worth watching for:

- **Object storage** once evidence-photo uploads (Phase 4+) are implemented.
- **A managed database** if losing this one VM would exceed your acceptable
  recovery window (the daily-backup policy mitigates but doesn't eliminate
  this).
- **A load balancer** only if/when this becomes more than one instance.
- **Cloudflare** (free tier) once a domain is in place — see
  docs/ARCHITECTURE.md; this also gets you free WAF-lite rules and DDoS
  absorption, which OCI's native WAF does not offer on Always Free.
