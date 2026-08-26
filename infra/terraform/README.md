# infra/terraform

Deploys FlockWatch's API to **GCP Cloud Run**, with **Neon** (neon.tech) for
Postgres/PostGIS. See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) for
the full rationale — in short, this replaced an original OCI/VM plan after
~40 failed `terraform apply` attempts over two days against OCI's Always
Free ARM capacity in `ca-toronto-1`. That OCI Terraform is still in this
repo (`modules/{network,compute,storage}`, `environments/prod`) but is
**dormant, not the active deployment target** — see the appendix at the
bottom if you want to revisit it.

## Layout

```
modules/
  cloud-run/   Cloud Run v2 service + AR remote-repo mirror + Secret Manager
environments/
  gcp/         Root module wiring the above together — the live deployment
  prod/        OCI (network/compute/storage) — dormant, see appendix
modules/{network,compute,storage}  OCI — dormant, see appendix
```

## One-time prerequisites

1. **A GCP project with billing linked.** GCP requires a payment method on
   file even for Always-Free-only usage (unlike OCI, where Always Free
   genuinely cannot bill you) — as long as usage stays within free tier
   limits, cost is $0, but the card requirement is worth knowing upfront.
   Console → Billing → create a billing account → link it to your project.
2. **`gcloud` CLI, authenticated**: `gcloud auth login` (for the CLI itself)
   and separately `gcloud auth application-default login` (for Terraform —
   these are two different credential grants). Enable the APIs this needs:
   ```
   gcloud services enable run.googleapis.com artifactregistry.googleapis.com \
     secretmanager.googleapis.com --project=<your-project-id>
   ```
3. **A Neon project** (neon.tech) with the connection string handy:
   ```
   npm install -g neonctl
   neonctl auth
   neonctl projects create --name flockwatch --org-id <your-org-id>
   ```
   Prints a `postgresql://...` connection URI — that's `database_url`. The
   API's own migration runner creates the PostGIS extension on first
   connect, so no manual Neon-side setup beyond creating the project.
4. **A published API image and its digest** —
   `.github/workflows/publish-api-image.yml` builds and pushes
   `services/api` to `ghcr.io/<owner>/flock-detector-api` on every push to
   `main` that touches `services/api/**`. Get the current digest (not the
   `:latest` tag — see "Why a digest, not a tag" below):
   ```
   docker pull ghcr.io/<owner>/flock-detector-api:latest
   ```
5. **A secret**: `openssl rand -base64 32` for `jwt_secret`.

## Deploying

```
cd infra/terraform/environments/gcp
cp terraform.tfvars.example terraform.tfvars   # fill in your real values
terraform init
terraform plan
terraform apply
```

**Never paste a connection string, key, or secret from this process into a
chat with an AI assistant, this repo, or anywhere else public** — the only
place `terraform.tfvars` should exist is your local machine (it's
gitignored).

### Why a digest, not a tag

`image_digest` is a required variable (`sha256:...`), not a default
`:latest`. This isn't just caution — it was a real bug found while
deploying: after pushing a fix and a fresh GHCR image, `terraform apply`
saw the same `:latest` string as before and silently skipped updating the
service. Even forcing it with `gcloud run deploy` using the same tag served
a **stale** build, because the Artifact Registry remote-repository mirror
in front of `ghcr.io` has its own tag-resolution cache, separate from
GHCR's. Deploying by digest means every apply deploys exactly the image you
chose — same reasoning as pinning GitHub Actions to a commit SHA rather
than a floating version tag.

## After `apply`

- `terraform output url` gives you the live HTTPS URL — TLS is automatic,
  no Caddy/cert management needed.
- `curl $(terraform output -raw url)/health` should return `{"status":"ok"}`.
  **Not `/healthz`** — Google's Cloud Run front-end infrastructure
  intercepts requests to that exact path before they reach the container
  (confirmed by comparing against `/`, `/deployments`, and an arbitrary
  unclaimed path, which all correctly reached the app). The app's health
  endpoint is named `/health` specifically to avoid this collision.

## Redeploying after a new image push

Get the new digest and re-apply with it:

```
docker pull ghcr.io/<owner>/flock-detector-api:latest   # prints the digest
# update image_digest in terraform.tfvars
terraform apply
```

## Cost notes

Cloud Run's `min_instance_count = 0` (scale to zero) means cost tracks
actual usage rather than a VM reserved 24/7 — at low/idle traffic, this
approaches $0. Neon's free tier covers Postgres/PostGIS with autosuspend.
Watch for:

- **Cloud Run's free egress allowance** is limited, similar in spirit to
  OCI's — real sustained traffic could incur charges.
- **Neon's free tier storage/compute-hour limits** if the dataset or query
  volume grows significantly past an MVP.
- **A custom domain + Cloudflare** aren't currently planned for this path
  (Cloud Run already provides managed TLS and doesn't expose a raw origin
  IP the way the OCI VM plan would have) — see docs/ARCHITECTURE.md if that
  changes.

---

## Appendix: OCI (dormant)

The original plan, kept in the repo as real work that may be worth
revisiting if Oracle's Always Free ARM capacity in a given region ever
frees up, but **not currently deployed or maintained as the active path**.

### Layout

```
modules/
  network/   VCN, subnet, internet gateway, security list
  compute/   The instance + its cloud-init bundle (compose file, Caddyfile)
  storage/   Persistent data volume + daily backup policy
environments/
  prod/      Root module wiring the three together
```

### One-time prerequisites

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
5. **Secrets**: `openssl rand -base64 24` for `db_password`,
   `openssl rand -base64 32` for `jwt_secret`.

### Deploying

```
cd infra/terraform/environments/prod
cp terraform.tfvars.example terraform.tfvars   # fill in your real values
terraform init
terraform plan
terraform apply
```

### Remote state backend — unverified, check before trusting

`versions.tf` has a commented-out `backend "oci" {}` block, Oracle's newer
native backend type (Terraform ≥ 1.12) for storing state in Object
Storage. **This was written without access to a real OCI account to test
against — verify it actually works on `terraform init` before relying on
it.** The fallback is the older, more battle-tested approach: an Object
Storage bucket's S3-compatible endpoint with the generic `s3` backend and
OCI Customer Secret Keys.

### Known blocker: Always Free capacity

`VM.Standard.A1.Flex` Always Free capacity was persistently exhausted in
`ca-toronto-1` across ~40 attempts over two days, including at the reduced
1 OCPU/6GB shape (`ocpus`/`memory_gbs` in `terraform.tfvars`). This is a
well-documented, common issue with OCI's Always Free ARM allocation,
particularly in smaller/newer regions. If retrying, a different region
(subject to your tenancy's region-subscription limit — Free Trial accounts
can be capped at just the home region) or a longer retry window may
eventually succeed.

### Day-2 config changes (important limitation)

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

### Destroying

```
terraform destroy
```

Note the data volume's daily backups (5/month included in Always Free) are
**not** automatically deleted by `destroy` unless they're tied to the
volume's lifecycle — check the OCI console under Block Storage → Backups
if you want to fully clean up a test environment.
