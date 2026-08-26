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

The importer, web app, and authentication are done; the mobile app is not yet built.

## Layout

```
apps/atlas      React web app (map, records, submit, review desk)  [done]
apps/mobile     React Native mobile app                     [not yet built]
services/api    Go REST API                                 [done]
services/importer  OSM/DeFlock bootstrap importer            [not yet built]
packages/shared-types  Shared TS types                       [not yet built]
infra/docker    Local dev Docker Compose                     [done]
infra/terraform GCP Cloud Run + Neon (live); OCI (dormant)   [done]
docs/           Architecture and planning docs
```
