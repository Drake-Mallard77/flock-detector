package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCoverage(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	published := seedRecord(t, "Detroit Police Department", "Detroit", "MI", "osm_documented")
	seedRecord(t, "Verified PD", "Somewhere", "TX", "confirmed")
	// Not published, so it must not inflate the totals.
	seedRecord(t, "Candidate PD", "Nowhere", "MT", "under_review")

	attachCameras(t, published, 3, "cov")
	// One more camera with no operator and no link — the gap the page exists
	// to report.
	if _, err := testPool.Exec(context.Background(), `
		INSERT INTO camera_sightings (location, source, status, external_id, state)
		VALUES (ST_SetSRID(ST_MakePoint(-84.0, 34.0), 4326)::geography,
		        'osm_import', 'confirmed', 'osm:node:cov:orphan', 'GA')
	`); err != nil {
		t.Fatalf("seed orphan camera: %v", err)
	}

	rec := doJSON(t, h, http.MethodGet, "/stats/coverage", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var c Coverage
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if c.Cameras != 4 {
		t.Errorf("expected 4 cameras, got %d", c.Cameras)
	}
	if c.CamerasLinked != 3 {
		t.Errorf("expected 3 linked cameras, got %d", c.CamerasLinked)
	}
	if c.PublishedRecords != 2 {
		t.Errorf("published should exclude the under_review record, got %d", c.PublishedRecords)
	}
	// The number the page exists to be honest about.
	if c.VerifiedRecords != 1 {
		t.Errorf("expected 1 verified record, got %d", c.VerifiedRecords)
	}
}
