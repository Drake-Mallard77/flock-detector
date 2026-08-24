# FlockWatch

A public-records atlas of Flock Safety / ALPR surveillance deployments across the US — web
app, then Android-first mobile apps, deployed to Oracle Cloud via Terraform.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full architecture and rollout plan,
[services/api/README.md](services/api/README.md) to run the backend locally, and
[infra/terraform/README.md](infra/terraform/README.md) to deploy to OCI.

## Status

Phase 1 (monorepo scaffold, Go API, Postgres/PostGIS schema, local Docker Compose) and Phase 2
(OCI Terraform, GHCR image publish) are done — see `docs/ARCHITECTURE.md`'s rollout section.
The OSM/DeFlock importer, the web app, real auth, and the mobile app are not yet built. A first
real `terraform apply` against a live OCI account is still pending.

## Layout

```
apps/web        React web app                              [not yet built]
apps/mobile     React Native mobile app                     [not yet built]
services/api    Go REST API                                 [done]
services/importer  OSM/DeFlock bootstrap importer            [not yet built]
packages/shared-types  Shared TS types                       [not yet built]
infra/docker    Local dev Docker Compose                     [done]
infra/terraform OCI Terraform (network/compute/storage)      [done]
docs/           Architecture and planning docs
```
