# apps/atlas

The FlockWatch public web app — React + TypeScript + Vite, with Leaflet over
proxied Esri raster tiles. Talks to the Go API in [`services/api`](../../services/api).

> **Why `apps/atlas` and not `apps/web`?** A separate ChatGPT/Sites integration with push
> access to this repo used `apps/web` for its own unrelated Next.js implementation. That
> directory has been removed, but the integration may recreate it — this app deliberately
> lives elsewhere so the two can not collide. See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md).

## Running

```bash
npm install
npm run dev        # http://localhost:5173
```

By default it points at the deployed Cloud Run API. To run against a local backend
(`infra/docker/docker-compose.yml`):

```bash
VITE_API_BASE=http://localhost:8080 npm run dev
```

Note the API's `ALLOWED_ORIGIN` must match the app's origin or requests will fail CORS —
locally that's `http://localhost:5173`, which is the API's default.

```bash
npm run typecheck  # tsc --noEmit
npm run build      # tsc -b && vite build
```

## Pages

| Route | What it does |
|---|---|
| `/` | Map of documented camera locations, clustered |
| `/deployments` | Agency-level records, filterable by review status |
| `/deployments/:id` | One record, its sources, and what's unknown about it |
| `/methodology` | How records are sourced/reviewed, plus ODbL attribution |
| `/submit` | Public submission form (lands in `under_review`) |

The Review Desk (moderator-only) is **not** here yet — it ships with real auth, replacing the
API's `dev-login` stub.

## Two things worth knowing before changing this

**The map loads by viewport, not all at once.** The API caps `/cameras` at 1,000 rows per
response, and a single state can hold several thousand (Illinois alone: ~5,000). The map
refetches the visible bbox on `moveend` (debounced) and says so plainly when a view is
truncated. Removing that and fetching once would silently show an arbitrary subset of the
country — which, for a project whose whole value is trustworthy data, is worse than showing
less.

**MapLibre is lazy-loaded.** It's ~1MB of the bundle; `App.tsx` loads it via `React.lazy` so
the text pages don't pay for it. Importing it statically anywhere would undo that (main bundle
goes 247KB → 1,310KB).

## Licensing obligation

Camera data derives from OpenStreetMap under **ODbL**, which requires attribution wherever it's
shown. The footer credit and the Methodology page's licensing section satisfy this — they are a
license term, not decoration. Don't remove them.
