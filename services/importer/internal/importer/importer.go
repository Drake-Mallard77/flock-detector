// Package importer upserts OSM-sourced ALPR camera nodes into
// camera_sightings.
//
// Scope note: this populates ONLY the camera_sightings (precise-pin)
// layer. It never writes a published deployment record. A deployments row
// is a public-records claim about a named agency — backed by a council
// report, contract, or FOIA response — whereas an OSM node is a
// crowdsourced map pin whose `operator` tag is a lead, not evidence.
//
// The sibling derive-deployments command turns those leads into
// under_review CANDIDATES for a human to confirm or reject. Nothing it
// produces is publicly visible until a moderator acts, so unverified
// claims about specific police departments never appear as fact.
package importer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"flockwatch/importer/internal/overpass"
)

// Rows per round trip. Large enough that a big state is tens of batches
// rather than thousands of round trips, small enough that a dropped
// connection loses little work and memory stays flat.
const batchSize = 500

type Stats struct {
	Fetched  int
	Inserted int
	Updated  int
	Skipped  int
}

// UpsertNodes writes nodes for one state. Idempotent: re-running an import
// updates existing rows (matched on external_id) rather than duplicating
// them, so the job can be re-run on a schedule to pick up new OSM edits.
//
// Imported rows land as status='confirmed', unlike user submissions which
// start 'under_review': OSM data has already been through OpenStreetMap's
// own community review, and holding 100k+ imported pins in a moderation
// queue would bury the genuinely-needs-review user submissions.
func UpsertNodes(ctx context.Context, pool *pgxpool.Pool, state string, nodes []overpass.Node) (Stats, error) {
	stats := Stats{Fetched: len(nodes)}

	// Batched rather than one round trip per node. Against a remote
	// serverless Postgres (Neon) the row-at-a-time version took over an
	// hour for a single large state and was killed partway through by a
	// dropped connection — thousands of sequential round trips is both slow
	// and a long window in which anything can interrupt.
	batch := &pgx.Batch{}
	flush := func() error {
		if batch.Len() == 0 {
			return nil
		}
		results := pool.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			var inserted bool
			if err := results.QueryRow().Scan(&inserted); err != nil {
				results.Close()
				return err
			}
			if inserted {
				stats.Inserted++
			} else {
				stats.Updated++
			}
		}
		if err := results.Close(); err != nil {
			return err
		}
		batch = &pgx.Batch{}
		return nil
	}

	for _, n := range nodes {
		if n.Lat == 0 && n.Lon == 0 {
			stats.Skipped++
			continue
		}

		externalID := "osm:node:" + strconv.FormatInt(n.ID, 10)

		var direction *int
		if d, ok := n.Tags["direction"]; ok {
			if v, err := strconv.Atoi(d); err == nil && v >= 0 && v <= 359 {
				direction = &v
			}
		}

		var cameraType *string
		// `camera:type` is the OSM tag for the physical camera style
		// (fixed/panning/dome); vendor product lines aren't in OSM tags, so
		// this is the closest available signal.
		if ct, ok := n.Tags["camera:type"]; ok && ct != "" {
			cameraType = &ct
		}

		// Normalized, not stored raw: OSM's free-text manufacturer tag
		// spells the same vendor several ways. NULL when absent or when the
		// tag itself says "unknown" — a gap is surfaced as a gap, matching
		// how the rest of the atlas treats missing data.
		manufacturer := NormalizeManufacturer(n.Tags["manufacturer"])

		// Who runs the camera, when OSM says. Vendor names are rejected —
		// see NormalizeOperator.
		operator := NormalizeOperator(n.Tags["operator"])

		batch.Queue(`
			INSERT INTO camera_sightings (
				location, direction, camera_type, manufacturer, operator,
				source, status, external_id, state
			) VALUES (
				ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
				$3, $4, $5, $6, 'osm_import', 'confirmed', $7, $8
			)
			ON CONFLICT (external_id) WHERE external_id IS NOT NULL
			DO UPDATE SET
				location     = EXCLUDED.location,
				direction    = EXCLUDED.direction,
				camera_type  = EXCLUDED.camera_type,
				manufacturer = EXCLUDED.manufacturer,
				operator     = EXCLUDED.operator,
				state        = EXCLUDED.state
			RETURNING (xmax = 0) AS inserted
		`, n.Lon, n.Lat, direction, cameraType, manufacturer, operator, externalID, state)

		if batch.Len() >= batchSize {
			if err := flush(); err != nil {
				return stats, fmt.Errorf("upsert batch: %w", err)
			}
		}
	}

	if err := flush(); err != nil {
		return stats, fmt.Errorf("upsert final batch: %w", err)
	}

	return stats, nil
}
