# services/importer

Bootstraps and refreshes the `camera_sightings` pin layer from OpenStreetMap via the Overpass
API. See [docs/ARCHITECTURE.md](../../docs/ARCHITECTURE.md) for how this fits the whole system.

## What it does (and deliberately doesn't)

Queries OSM for nodes tagged `man_made=surveillance` + `surveillance:type=ALPR` +
`manufacturer=Flock Safety`, one US state at a time, and upserts them into `camera_sightings`.

It populates **only** the precise-pin layer, never `deployments`. A `deployments` row is a
public-records claim about a named agency, backed by a council report, contract, or FOIA
response. An OSM node is a crowdsourced map pin — its `operator` tag is a hint, not evidence.
Auto-promoting those into agency-level records would put unverified claims about specific
police departments into the part of the site that presents itself as records-backed. A
moderator can link OSM cameras to a deployment by hand.

Imported rows land as `status='confirmed'` (unlike user submissions, which start
`under_review`): OSM data has already been through OpenStreetMap's own community review, and
holding 100k+ imported pins in a moderation queue would bury the submissions that genuinely
need review.

## Licensing — read before redistributing

OSM data is **ODbL**. Any redistribution of the resulting database must credit
**"© OpenStreetMap contributors"** and keep the derived database open under the same terms.
This attribution must be surfaced on the app's Methodology page before launch — it is not
optional.

## Usage

```bash
export DATABASE_URL="postgres://flockwatch:flockwatch@localhost:5432/flockwatch?sslmode=disable"

go run .                      # all 50 states + DC
go run . -states CA,TX,NY     # specific states
go run . -dry-run -states DC  # fetch and report counts, write nothing
go run . -delay 30s           # slow down further if rate limited
```

Re-running is safe and expected: rows are matched on `external_id` (`osm:node:<id>`) and
updated in place, never duplicated, so this can run on a schedule to pick up new OSM edits.

## Scale and rate limits

Real numbers from testing: DC has ~86 cameras, California alone has ~14,900. A full US import
is on the order of 100k+ rows.

The public Overpass instance rate-limits aggressively — a 429 was observed on back-to-back
large-state queries even at 5s spacing. The client handles this with exponential backoff
(30s → 60s → 120s, 4 attempts), and the default inter-state delay is 15s. A state that still
fails after retries is logged and skipped rather than aborting the run; the next run picks it
up. Expect a full US import to take a while — this is a bulk backfill, not an interactive job.

Please don't lower `-delay` or strip the `User-Agent` — Overpass is a free shared service and
abusing it risks getting the importer blocked outright.
