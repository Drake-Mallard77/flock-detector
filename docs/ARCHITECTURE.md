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
    terraform/      OCI network/compute/storage modules                   [Phase 2]
    docker/         Dockerfiles + docker-compose (local dev is done; a     [Phase 1 — done (dev);
                     prod compose with Caddy lands with Terraform)          Phase 2 (prod)]
    cloud-init/     VM bootstrap script for the OCI instance                [Phase 2]
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

## Infrastructure (OCI, Terraform) — Phase 2, not yet built

- Compute: `oci_core_instance`, shape `VM.Standard.A1.Flex`, `ocpus=2, memory_in_gbs=12`
  (current Always Free ARM allocation), cloud-init installs Docker and brings up the compose
  stack (adds Caddy for automatic HTTPS in front of `api` + the built `web` static assets).
- Network: `oci_core_vcn` + subnet + internet gateway + security list (80/443/22 only).
- Storage: `oci_core_volume` (+ attachment) for a persistent Postgres data volume, with
  `oci_core_volume_backup_policy` for nightly backups; `oci_objectstorage_bucket` for
  user-uploaded evidence photos.
- Remote state: native `oci` Terraform backend, state in an Object Storage bucket.
- Known risk: A1.Flex "out of host capacity" errors are common on Always Free — the apply
  runbook documents a retry/alternate-AD fallback.

## Local development

```
cd infra/docker
docker compose up --build
```

Brings up Postgres+PostGIS and the API on `:8080` (migrations run automatically on API
startup). See `services/api/README.md` for endpoint details and a dev-login example.

## Rollout phases

1. **Monorepo scaffold + Go API + Postgres/PostGIS schema + local Docker Compose — done.**
2. Terraform (network/compute/storage) + cloud-init → first reachable OCI deployment.
3. OSM/DeFlock importer.
4. Web app (map, deployments list/detail, methodology, submission form).
5. Real auth (replacing the dev-login stub) + Review Desk moderation UI.
6. React Native mobile app (Android-first).
7. Phase 2+: push notifications, iOS polish, richer analytics.
