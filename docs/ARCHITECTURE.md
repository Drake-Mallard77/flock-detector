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

## Bootstrap data: OSM / DeFlock — Phase 3, done

`services/importer` (see its [README](../services/importer/README.md)). OSM nodes tagged
`man_made=surveillance` + `surveillance:type=ALPR` are fetched
via Overpass QL (`https://overpass-api.de/api/interpreter`), one US state at a time, and
upserted into `camera_sightings` keyed on `external_id` (`osm:node:<id>`) so re-runs update
rather than duplicate.

**Populates only `camera_sightings`, never `deployments`** — a deployments row is a
public-records claim about a named agency; an OSM node is a crowdsourced pin whose `operator`
tag is a hint, not evidence. Auto-promoting those would put unverified claims about specific
police departments into the records-backed part of the site. Moderators can link them by hand.

Imported rows land `status='confirmed'` (user submissions start `under_review`): OSM data has
already been through OpenStreetMap's community review, and queueing 100k+ pins for moderation
would bury the submissions that actually need it.

OSM data is **ODbL** — the derived database must stay attributable ("© OpenStreetMap
contributors"); share-alike applies to the database itself, not just the rendered map. The
attribution is live: in the footer on every page, and in the Methodology page's licensing
section.

Real scale, measured: DC ~86 cameras, California ~14,900; a full US import is 100k+ rows.
The public Overpass instance rate-limits aggressively (429 on back-to-back large-state queries
even at 5s spacing), so the client retries with exponential backoff and defaults to a 15s
inter-state delay.

## Services

```
flock-detector/
  apps/
    web/            React + TS + Vite, MapLibre GL JS + OSM tiles          [Phase 4]
    mobile/         React Native (Expo) — Android-first rollout            [Phase 6]
  services/
    api/            Go backend (chi router), REST JSON API                 [Phase 1 — done]
    importer/       Overpass fetch -> camera_sightings upsert (ODbL)      [Phase 3 — done]
  packages/
    shared-types/   OpenAPI schema -> generated TS types, web+mobile       [Phase 4]
  infra/
    terraform/      modules/cloud-run + environments/gcp — live deploy    [Phase 2 — done]
                    modules/{network,compute,storage} + environments/prod
                    — OCI, dormant (Always Free capacity never freed up)
    docker/         Local dev Dockerfile + docker-compose                 [Phase 1 — done]
  docs/
```

### `services/api` (Go)

- Router: `go-chi`. Endpoints: `GET/POST /deployments`, `GET /deployments/{id}`,
  `POST /deployments/{id}/review` (moderator/admin only), `GET/POST /cameras`.
- Auth: Google Sign-In exchanged for our own JWT (`golang-jwt/v5`), role-gated middleware.
  The Google ID token is *validated* — signature against Google's JWKS, plus issuer, expiry,
  and audience — rather than merely decoded, and the role always comes from our database.
  Signing in proves an email address and nothing more. Roles are granted only through the
  `grant-role` CLI, which requires database credentials; there is deliberately no web path to
  moderator access. `requireRole` re-reads the role per request, so revocation takes effect
  immediately instead of when a 24h token expires. `POST /auth/dev-login` survives as a
  local-development stub and is never mounted when `ENV=production`.
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

## Infrastructure — GCP Cloud Run + Neon (live), OCI (dormant)

`infra/terraform/{modules/cloud-run,environments/gcp}`. See
[infra/terraform/README.md](../infra/terraform/README.md) for the operational how-to.

**Live**: `https://theflockwatcher.com` (web) and `https://api.theflockwatcher.com` (API),
both Cloudflare-proxied in front of Cloud Run. See "Domains and edge" below.

The original plan was OCI (`VM.Standard.A1.Flex` Always Free) — that Terraform
(`infra/terraform/{modules/{network,compute,storage},environments/prod}`) is left in place, not
deleted, but is **not the active deployment target**: ~40 `terraform apply` attempts over two
days all failed with "out of host capacity" in `ca-toronto-1`, a well-documented Always Free
ARM shortage that showed no sign of clearing. Pivoted to GCP:

- **Cloud Run** (serverless containers), not a VM: no reserved-capacity pool to run out of —
  Google allocates per-request rather than reserving a slot ahead of time. `min_instance_count
  = 0` (scale to zero), so cost approaches $0 at low/idle traffic rather than paying for a VM
  provisioned 24/7. First real apply succeeded in under a minute.
- **Neon** (neon.tech) for Postgres+PostGIS instead of a self-hosted container: serverless,
  free tier, autosuspends when idle. Removes the entire class of problems the OCI path needed
  Terraform for (persistent volume, backup policy, cloud-init mount scripting) — the database
  isn't infrastructure we manage at all anymore.
- Image: pulled from GHCR through an **Artifact Registry remote repository** (`ghcr-mirror`),
  not directly — Google's docs recommend this for reliability over a direct public-GHCR pull.
  Deployed **by digest** (`image_digest` variable, `sha256:...`), not the `:latest` tag: a
  floating-tag deploy silently failed to pick up a fresh push (Terraform saw no string diff on
  the image reference; even a forced `gcloud run deploy` with the same tag served a stale build
  from the AR mirror's own tag cache). Same reasoning as pinning GitHub Actions to a commit SHA.
- `DATABASE_URL`/`JWT_SECRET` in **Secret Manager**, injected via `secret_key_ref`, not plain
  Cloud Run env vars.
- **Known Cloud Run quirk**: the health endpoint is `/health`, not `/healthz` — Google's
  front-end infrastructure intercepts requests to the literal path `/healthz` before they reach
  the container (confirmed by comparing against `/`, `/deployments`, and an arbitrary unclaimed
  path, which all correctly reached the app). Renamed throughout rather than worked around.
- Remote state: not yet set up (local `terraform.tfstate`, gitignored) — same "get the first
  real deploy working before debugging a backend" reasoning as the OCI path originally had.
- GCP requires a billing account (payment method) linked even for Always-Free-only usage,
  unlike OCI where Always Free genuinely cannot bill — not a cost concern as long as usage
  stays within free tier, just worth knowing going in.

### Cloudflare — not currently planned for this path

The earlier plan to front OCI with Cloudflare (WAF, DDoS absorption, hiding the origin IP) was
specific to self-hosting on a VM with a public IP. Cloud Run already provides managed TLS and
sits behind Google's own front-end infrastructure rather than exposing a raw origin IP, so the
original rationale doesn't carry over directly — revisit if/when this needs edge-level
rate limiting or WAF rules beyond what Cloud Run itself offers.

### Two parallel implementations exist in this repo — don't build on `apps/web`

A ChatGPT/OpenAI Sites integration connected to this GitHub repo independently pushes and
merges its own implementation under `apps/web/` (Next.js/vinext, Cloudflare D1, Drizzle ORM) —
matching the live `flockwatch-us.simwes07.chatgpt.site` prototype. It is **not** part of this
architecture: different backend, different cloud platform, not reviewed the way the Go/GCP side
has been. Confirmed direction (2026-08-25): the Go + GCP stack documented here is the actual
product; `apps/web`'s content should be left alone, not extended or depended on. It may
reappear via a future auto-merge from that integration — if so, treat it the same way: leave it
untouched, don't rebase this architecture around it.

## Local development

```
cd infra/docker
docker compose up --build
```

Brings up Postgres+PostGIS and the API on `:8080` (migrations run automatically on API
startup). See `services/api/README.md` for endpoint details and a dev-login example.


## Domains and edge (live)

- `theflockwatcher.com` and `www` — the web app, Cloudflare-proxied in front of Cloud Run.
- `api.theflockwatcher.com` — the API, same arrangement.

Cloudflare sits in front for WAF, DDoS absorption, and to keep the Cloud Run
origin off the public record. Getting there involved two dead ends worth
recording so nobody repeats them:

1. **Host Header Override is not on Cloudflare's free plan.** The original
   plan was a proxied CNAME plus an Origin Rule rewriting the Host, which
   would have avoided Google's domain verification entirely. The API returns
   `not entitled to use the HostHeader override`. Cloud Run domain mappings
   (and therefore Search Console verification) are required.
2. **Certificates cannot be issued while Cloudflare proxies the record.**
   Google validates by connecting to the domain; with the orange cloud on,
   Cloudflare terminates TLS and the validation never reaches Cloud Run,
   producing a persistent HTTP 525. Each hostname must be set to DNS-only
   until its certificate is provisioned, then re-proxied.

**Whenever an origin changes, more than DNS needs updating.** Two allowlists
broke on the cutover, both failing in ways the server logs don't show:

- **CORS** (`ALLOWED_ORIGIN`, a comma-separated list built from
  `site_domain` in Terraform). A missing origin means every data request is
  blocked in the browser while the server logs a clean 200.
- **Google OAuth authorized JavaScript origins**, which are console-only on
  a personal (non-org) account — `gcloud` cannot manage them. A missing
  origin gives `Error 400: origin_mismatch` at sign-in.

## Scheduled jobs

Cloud Run Jobs on Cloud Scheduler, weekly:

- `flockwatch-refresh-cameras` (Mon 04:00 UTC) — re-imports all 51
  jurisdictions from Overpass. Idempotent, so re-runs update rather than
  duplicate.
- `flockwatch-derive-deployments` (Mon 06:00 UTC) — two hours later, so it
  works from freshly imported operator tags.

Jobs rather than a GitHub Actions cron specifically to keep the database
credential in Secret Manager instead of copying it into a repo secret.

## Record statuses

`confirmed` means a moderator checked the record against a council report,
contract, or FOIA response. `osm_documented` means OpenStreetMap
contributors mapped the cameras and attributed them to that operator —
real, citable, and **not** a public record.

The distinction exists because ~1,150 records are OSM-derived and none have
been verified. Publishing them as `confirmed` on a site titled "Public
Records Atlas" would assert exactly the verification the project exists to
provide. Anything derived starts `under_review` and only a human moves it.

## Rollout phases

1. **Monorepo scaffold + Go API + Postgres/PostGIS schema + local Docker Compose — done.**
2. **Terraform + GHCR image publish + first real deploy — done, on GCP Cloud Run + Neon**
   (pivoted from OCI after ~40 failed applies against Always Free ARM capacity — see above).
   Live at `https://flockwatch-api-wlfs54kbla-uc.a.run.app`.
3. **OSM/DeFlock importer — done** (`services/importer`). Verified end-to-end against the real
   Overpass API and a local Postgres: 86 DC cameras imported, re-run confirmed idempotent.
   Not yet run against the live Neon database.
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
