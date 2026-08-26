# FlockWatch

A public-records atlas of Flock Safety / ALPR surveillance deployments across the US — web
app, then Android-first mobile apps, backend deployed to GCP Cloud Run + Neon via Terraform.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full architecture and rollout plan,
[services/api/README.md](services/api/README.md) to run the backend locally, and
[infra/terraform/README.md](infra/terraform/README.md) to deploy.

## Status

Phase 1 (monorepo scaffold, Go API, Postgres/PostGIS schema, local Docker Compose) and Phase 2
(Terraform, GHCR image publish, first live deploy) are done — see `docs/ARCHITECTURE.md`'s
rollout section. **Live API**: `https://flockwatch-api-wlfs54kbla-uc.a.run.app`.

The deployment target is GCP Cloud Run + Neon, not the originally-planned OCI VM — pivoted
after OCI's Always Free ARM capacity stayed exhausted across ~40 apply attempts over two days.
The OCI Terraform is still in the repo but dormant; see `infra/terraform/README.md`.

The OSM/DeFlock importer, the web app, real auth, and the mobile app are not yet built. Note:
`apps/web` contains an unrelated, independently-maintained implementation pushed by a separate
ChatGPT/Sites integration connected to this repo — it is not part of this architecture, see
`docs/ARCHITECTURE.md`.

## Layout

```
apps/web        Unrelated Sites-managed app — not part of this architecture, do not build on it
apps/mobile     React Native mobile app                     [not yet built]
services/api    Go REST API                                 [done]
services/importer  OSM/DeFlock bootstrap importer            [not yet built]
packages/shared-types  Shared TS types                       [not yet built]
infra/docker    Local dev Docker Compose                     [done]
infra/terraform GCP Cloud Run + Neon (live); OCI (dormant)   [done]
docs/           Architecture and planning docs
```
