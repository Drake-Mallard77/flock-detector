# services/api

Go REST API for FlockWatch. See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) for the
full picture.

## Run locally

```
cd infra/docker
docker compose up --build
```

Or without Docker (requires a local Postgres+PostGIS reachable at `DATABASE_URL`):

```
cd services/api
go run .
```

Migrations in `migrations/*.sql` run automatically on startup.

## Try it

```bash
# Health check
curl http://localhost:8080/health

# Submit a deployment (lands in under_review)
curl -X POST http://localhost:8080/deployments -H 'Content-Type: application/json' -d '{
  "agency_name": "Springfield PD",
  "city": "Springfield",
  "state": "IL",
  "evidence_type": "council_report",
  "source_links": ["https://example.gov/council-minutes.pdf"]
}'

# List deployments
curl http://localhost:8080/deployments

# Dev-only: get a moderator token (stub auth, see docs/ARCHITECTURE.md)
curl -X POST http://localhost:8080/auth/dev-login -H 'Content-Type: application/json' -d '{
  "email": "mod@example.com", "role": "moderator"
}'

# Approve it as a moderator
curl -X POST http://localhost:8080/deployments/<id>/review \
  -H "Authorization: Bearer <token>" -H 'Content-Type: application/json' \
  -d '{"status": "confirmed"}'
```

## Environment variables

| Variable        | Default                                                              |
|-----------------|-----------------------------------------------------------------------|
| `PORT`          | `8080`                                                                 |
| `DATABASE_URL`  | `postgres://flockwatch:flockwatch@localhost:5432/flockwatch?sslmode=disable` |
| `JWT_SECRET`    | `dev-secret-change-me` (**must** be overridden outside development)   |
| `ALLOWED_ORIGIN`| `http://localhost:5173`                                               |
| `ENV`           | `development` — set to `production` to disable `/auth/dev-login`      |
