# FlockWatch — Architecture

## What this is

FlockWatch documents Flock Safety / ALPR surveillance deployments across the US: a public
records atlas (agency, city, evidence, review status), not a raw exact-GPS camera map. The
product direction was validated by a working prototype at
`flockwatch-us.simwes07.chatgpt.site` ("Public Records Atlas") — its IA (Map / Deployments /
Methodology / Submit a sighting / Review Desk) and data shape are the source of truth this
rebuild is matching, now on infrastructure we own so it can be deployed reliably via Terraform.

## Data model

**`Deployment`** — the primary, agency/contract-level record shown by default, at city-level
precision:
`agency_name, city, state, county, location (city-level), documented_units (nullable),
evidence_type, source_links[], status, notes, reviewed_by, last_reviewed_at`.

Status lifecycle: `under_review` (default on submission) → `confirmed` / `contract_found` /
`disputed` / `removed`, set by a moderator via the Review Desk.

**`CameraSighting`** — an opt-in, precise-pin layer (not shown by default): exact `lat/lng`,
`direction`, `camera_type`, `photo_url`, optionally linked to a `Deployment`. Populated by user
submissions and by the OSM/DeFlock bootstrap import (`source = osm_import`).

**Roles**: `submitter` (anonymous or logged in; submissions land in `under_review`),
`moderator` (Review Desk — approve/edit/reject), `admin`.

## Bootstrap data: OSM / DeFlock

OSM nodes tagged `man_made=surveillance` + `surveillance:type=ALPR` +
`manufacturer=Flock Safety` are fetched via Overpass QL (`https://overpass-api.de/api/interpreter`),
scoped per US state. OSM data is **ODbL** — the derived database must stay attributable
("© OpenStreetMap contributors", shown on the Methodology page); share-alike applies to the
database itself, not just the rendered map.

## Services

```
flock-detector/
  apps/
    web/            React + TS + Vite, MapLibre GL JS + OSM tiles          [Phase 4]
    mobile/         React Native (Expo) — Android-first rollout            [Phase 6]
  services/
    api/            Go backend (chi router), REST JSON API                 [Phase 1 — done]
    importer/       Overpass fetch + ODbL attribution + seed/refresh job   [Phase 3]
  packages/
    shared-types/   OpenAPI schema -> generated TS types, web+mobile       [Phase 4]
  infra/
    terraform/      OCI network/compute/storage modules                   [Phase 2 — done]
    docker/         Local dev Dockerfile + docker-compose                 [Phase 1 — done]
                    (prod compose/Caddyfile live inside the compute
                    module's own template bundle, not here — see below)
  docs/
```

### `services/api` (Go)

- Router: `go-chi`. Endpoints: `GET/POST /deployments`, `GET /deployments/{id}`,
  `POST /deployments/{id}/review` (moderator/admin only), `GET/POST /cameras`.
- Auth: JWT (`golang-jwt/v5`), role-gated middleware. **`POST /auth/dev-login` is a stub**
  (upserts a user by email, issues a JWT for a requested role, no verification) that exists
  only to unblock local testing of the moderator-gated endpoints before real email
  magic-link/OAuth ships in Phase 5. It is only mounted when `ENV != production`
  (`config.DevAuthEnabled`) — real auth must replace it before this goes live.
- DB: Postgres 16 + PostGIS, `geography(Point,4326)` + GiST index for bbox queries. A minimal
  embedded SQL migration runner (`internal/db/migrate.go`) applies `migrations/*.sql` in
  filename order, tracked in a `schema_migrations` table — no external migration tool
  dependency.
- Security hardening (this documents specific law-enforcement agencies and a specific vendor,
  and should assume it will be targeted): refuses to boot in production with the default
  `JWT_SECRET` (`config.RequireSecureSecrets`); JWT verification pinned to HS256
  (`jwt.WithValidMethods`) against algorithm-confusion attacks; per-IP rate limiting on
  `POST /deployments` and `POST /cameras` (`internal/httpapi/ratelimit.go`) — framed as a
  data-integrity control (flooding fake submissions to discredit/dilute the dataset), not just
  abuse prevention. Test suite (`internal/httpapi/*_test.go`) uses testcontainers-go against
  real Postgres+PostGIS, not mocks.

## Infrastructure (OCI, Terraform) — Phase 2, done

`infra/terraform/{modules/{network,compute,storage},environments/prod}`. See
[infra/terraform/README.md](../infra/terraform/README.md) for the operational how-to
(prerequisites, deploying, day-2 config changes, destroying).

- Compute: `oci_core_instance`, shape `VM.Standard.A1.Flex`, `ocpus=2, memory_in_gbs=12`
  (current Always Free ARM allocation). Cloud-init (self-contained inside the compute module's
  `templates/` dir) installs Docker via the official apt repo (GPG-verified, not curl|sh),
  mounts the separately-attached data volume, and brings up a compose stack of `api` +
  `postgis/postgis` + Caddy (automatic HTTPS once a domain is set; plain HTTP on the bare IP
  until then).
- Network: `oci_core_vcn` + subnet + internet gateway + security list. Port 80/443 source
  CIDRs default to the whole internet (nothing fronts the origin yet) but are a variable
  (`http_source_cidrs`) specifically so they can be narrowed to Cloudflare's published ranges
  once Cloudflare is in front — see "Planned: Cloudflare" below. SSH is restricted to a
  required `ssh_source_cidr` (your IP, not `0.0.0.0/0`).
- Storage: a dedicated `oci_core_volume` (separate from the boot volume) + paravirtualized
  attachment, so Postgres data survives instance recreation, with
  `oci_core_volume_backup_policy` for daily backups (5/month included in Always Free).
- Images: `.github/workflows/publish-api-image.yml` builds & pushes `services/api` to
  `ghcr.io/<owner>/flock-detector-api` on changes to `services/api/**`; cloud-init pulls this.
- Remote state: `backend "oci" {}` (Terraform ≥ 1.12's native backend) — **unverified against a
  real account while building this**, flagged clearly in `versions.tf` and the Terraform
  README, with the older S3-compatible-endpoint approach documented as a fallback.
- Known risk: A1.Flex "out of host capacity" errors are common on Always Free — the Terraform
  README documents a retry/alternate-AD fallback.

### Planned: Cloudflare in front of the origin

Confirmed direction, not yet built (needs a domain + Cloudflare account, which this session
doesn't have): once a domain exists, front the OCI instance with Cloudflare's free tier — free
WAF-lite rules, DDoS absorption, edge rate limiting, and it hides the origin IP entirely (OCI's
own WAF/WAAS is a paid add-on, not available on Always Free). At that point,
`http_source_cidrs` should be narrowed from `0.0.0.0/0` to Cloudflare's published IP ranges
(https://www.cloudflare.com/ips/) so the origin only accepts traffic that's already passed
through Cloudflare's layer.

## Local development

```
cd infra/docker
docker compose up --build
```

Brings up Postgres+PostGIS and the API on `:8080` (migrations run automatically on API
startup). See `services/api/README.md` for endpoint details and a dev-login example.

## Rollout phases

1. **Monorepo scaffold + Go API + Postgres/PostGIS schema + local Docker Compose — done.**
2. **Terraform (network/compute/storage) + cloud-init + GHCR image publish — done.** (First
   real `terraform apply` against a live OCI account is still pending — see the Terraform
   README's prerequisites.)
3. OSM/DeFlock importer.
4. Web app (map, deployments list/detail, methodology, submission form).
5. Real auth (replacing the dev-login stub) + Review Desk moderation UI.
6. React Native mobile app (Android-first).
7. Phase 2+: push notifications, iOS polish, richer analytics, Cloudflare in front of the
   origin (see above).

## Repo/CI security posture

- Actions pinned to commit SHAs, not floating tags; `sha_pinning_required` enforced repo-wide;
  Actions restricted to GitHub-owned/verified-creator sources
  (`.github/workflows/*.yml`, repo Settings → Actions).
- `api-ci.yml` (build/vet/gofmt/test/govulncheck), `codeql.yml` (static analysis,
  push/PR/weekly), `dependency-review.yml` (blocks high-severity/incompatible deps on PRs),
  `dependabot.yml` (weekly gomod/Docker-base-image/Action-SHA updates).
- Secret scanning + push protection, Dependabot alerts + automated security-fix PRs: enabled at
  the repo level (free on this public repo).
