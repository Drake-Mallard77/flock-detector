# FlockWatch

A public-records atlas of Flock Safety / ALPR surveillance deployments across the US — web
app, then Android-first mobile apps, deployed to Oracle Cloud via Terraform.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full architecture and rollout plan,
and [services/api/README.md](services/api/README.md) to run the backend locally.

## Status

Phase 1 (monorepo scaffold, Go API, Postgres/PostGIS schema, local Docker Compose) is done.
Terraform/OCI deployment, the OSM/DeFlock importer, the web app, real auth, and the mobile app
are not yet built — see the rollout plan in `docs/ARCHITECTURE.md`.

## Layout

```
apps/web        React web app                      [not yet built]
apps/mobile     React Native mobile app             [not yet built]
services/api    Go REST API                         [done]
services/importer  OSM/DeFlock bootstrap importer   [not yet built]
packages/shared-types  Shared TS types              [not yet built]
infra/docker    Local dev Docker Compose            [done]
infra/terraform OCI Terraform (network/compute/storage)  [not yet built]
infra/cloud-init  OCI VM bootstrap script            [not yet built]
docs/           Architecture and planning docs
```
