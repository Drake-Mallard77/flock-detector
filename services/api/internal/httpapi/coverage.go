package httpapi

import (
	"net/http"
	"time"
)

// Coverage: what the atlas holds, and what it doesn't know.
//
// Every figure here is computed rather than written down, because a
// hand-maintained "about the data" page is wrong within a week of being
// published and nobody notices. The uncomfortable numbers are included on
// purpose — the share of cameras nobody can attribute to an agency, and the
// share of records nobody has checked against a public record. A
// transparency project that only publishes its strong numbers is asking for
// trust it hasn't shown the working for.

type Coverage struct {
	Cameras int `json:"cameras"`
	// CamerasWithOperator is how many carry an operator tag in
	// OpenStreetMap. The rest cannot be attributed to any agency, which is
	// the single largest gap in the dataset.
	CamerasWithOperator  int `json:"cameras_with_operator"`
	CamerasWithDirection int `json:"cameras_with_direction"`
	// CamerasLinked is how many are attached to a published record. Lower
	// than CamerasWithOperator, because an operator only helps if it also
	// matches an agency the atlas has a record for.
	CamerasLinked int `json:"cameras_linked"`

	PublishedRecords int `json:"published_records"`
	// VerifiedRecords is those checked against a council report, contract,
	// invoice, news article, or FOIA response. The remainder are derived
	// from OpenStreetMap and labelled as such.
	VerifiedRecords int `json:"verified_records"`

	States int `json:"states"`
	// LastImport is when the weekly refresh last touched camera data. A
	// reader deciding whether to rely on this needs to know how old it is.
	LastImport *time.Time `json:"last_import,omitempty"`
}

// handleCoverage serves GET /stats/coverage.
func (s *Server) handleCoverage(w http.ResponseWriter, r *http.Request) {
	var c Coverage

	// One round trip. These are all cheap aggregates, and issuing eight
	// queries to build one small object would make the page slower than the
	// data it describes.
	err := s.db.QueryRow(r.Context(), `
		SELECT
			(SELECT count(*) FROM camera_sightings),
			(SELECT count(*) FROM camera_sightings WHERE operator IS NOT NULL),
			(SELECT count(*) FROM camera_sightings WHERE direction IS NOT NULL),
			(SELECT count(*) FROM camera_sightings WHERE deployment_id IS NOT NULL),
			(SELECT count(*) FROM deployments WHERE status = ANY($1)),
			(SELECT count(*) FROM deployments WHERE status = ANY($2)),
			(SELECT count(DISTINCT state) FROM camera_sightings WHERE state IS NOT NULL),
			(SELECT max(updated_at) FROM camera_sightings WHERE source = 'osm_import')
	`, publishedStatuses, []string{"confirmed", "contract_found"}).Scan(
		&c.Cameras, &c.CamerasWithOperator, &c.CamerasWithDirection, &c.CamerasLinked,
		&c.PublishedRecords, &c.VerifiedRecords, &c.States, &c.LastImport,
	)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load coverage figures", err)
		return
	}

	// Moves only when the weekly import runs.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, c)
}
