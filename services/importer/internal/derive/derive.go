// Package derive turns OSM operator tags into deployment CANDIDATES for
// human review.
//
// The distinction this package exists to protect: a deployment record is a
// public-records claim that a named agency runs ALPR cameras in a named
// place. An OSM operator tag is a crowdsourced lead. Publishing the second
// as the first would put unverified accusations about specific police
// departments into the part of the site that presents itself as
// records-backed — the single worst failure mode this project has.
//
// So everything here lands as status='under_review' with
// evidence_type='osm_import', which is not publicly listed as an
// established record. A moderator confirms or rejects each one.
package derive

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"flockwatch/importer/internal/importer"
)

type Candidate struct {
	Operator string
	State    string
	Cameras  int
	Lat      float64
	Lng      float64
}

type Stats struct {
	Groups   int
	Created  int
	Existing int
	Skipped  int
}

// MinCameras is the floor for proposing a candidate. A single tagged camera
// is too thin a basis to raise an agency-level record about a named police
// department, and reviewing hundreds of one-camera leads would bury the
// substantial ones.
const MinCameras = 3

// FindCandidates groups imported cameras by operator and state.
//
// The centroid is only used to place the record on a map and to look up a
// city name; it is not a claim that anything is at that point.
func FindCandidates(ctx context.Context, pool *pgxpool.Pool) ([]Candidate, error) {
	rows, err := pool.Query(ctx, `
		SELECT operator,
		       state,
		       count(*) AS cameras,
		       ST_Y(ST_Centroid(ST_Collect(location::geometry))) AS lat,
		       ST_X(ST_Centroid(ST_Collect(location::geometry))) AS lng
		FROM camera_sightings
		WHERE operator IS NOT NULL
		  AND state IS NOT NULL
		  AND source = 'osm_import'
		GROUP BY operator, state
		HAVING count(*) >= $1
		ORDER BY count(*) DESC
	`, MinCameras)
	if err != nil {
		return nil, fmt.Errorf("group by operator: %w", err)
	}
	defer rows.Close()

	var out []Candidate
	for rows.Next() {
		var c Candidate
		if err := rows.Scan(&c.Operator, &c.State, &c.Cameras, &c.Lat, &c.Lng); err != nil {
			return nil, fmt.Errorf("scan candidate: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Create inserts one candidate as an under_review deployment.
//
// Idempotent on (agency_name, state): re-running never duplicates a record,
// and — importantly — never resurrects or overwrites one a moderator has
// already confirmed, disputed, or removed. A rejected candidate stays
// rejected across weekly runs rather than reappearing in the queue forever.
func Create(ctx context.Context, pool *pgxpool.Pool, c Candidate, city string) (created bool, err error) {
	// Classified so the record states what kind of body this is. Keyword
	// matching is fallible, which is exactly why the result is a candidate
	// for review rather than a published record.
	operatorType := importer.ClassifyOperator(c.Operator)

	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM deployments
			WHERE lower(agency_name) = lower($1) AND state = $2
		)
	`, c.Operator, c.State).Scan(&exists); err != nil {
		return false, fmt.Errorf("check existing: %w", err)
	}
	if exists {
		return false, nil
	}

	// documented_units is the count of OSM-tagged cameras attributed to this
	// operator — deliberately NOT presented as a figure from public records.
	// evidence_type='osm_import' is what tells a reader (and the moderator)
	// which of those two it is.
	_, err = pool.Exec(ctx, `
		INSERT INTO deployments (
			agency_name, city, state, location,
			documented_units, evidence_type, source_links, status, notes,
			operator_type
		) VALUES (
			$1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography,
			$6, 'osm_import', $7, 'under_review', $8, $9
		)
	`, c.Operator, city, c.State, c.Lng, c.Lat, c.Cameras,
		[]string{"https://www.openstreetmap.org/copyright"},
		fmt.Sprintf(
			"Candidate derived from OpenStreetMap: %d camera node(s) in %s carry operator=%q. "+
				"Not verified against public records — confirm against a council report, contract, "+
				"or FOIA response before publishing.", c.Cameras, c.State, c.Operator),
		string(operatorType),
	)
	if err != nil {
		return false, fmt.Errorf("insert candidate: %w", err)
	}
	return true, nil
}
