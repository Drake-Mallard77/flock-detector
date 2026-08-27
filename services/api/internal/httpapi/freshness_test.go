package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestDataFreshness(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	// No imported data at all is a legitimate failure, not an OK.
	rec := doJSON(t, h, http.MethodGet, "/health/data", nil, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("with no data: expected 503, got %d", rec.Code)
	}

	seed := func(age time.Duration) {
		t.Helper()
		if _, err := testPool.Exec(context.Background(), `
			INSERT INTO camera_sightings (location, source, status, external_id, updated_at)
			VALUES (ST_SetSRID(ST_MakePoint(-89.6, 39.8), 4326)::geography,
			        'osm_import', 'confirmed', $1, now() - $2::interval)
			ON CONFLICT (external_id) WHERE external_id IS NOT NULL
			DO UPDATE SET updated_at = EXCLUDED.updated_at
		`, "osm:node:freshness", age.String()); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// A recent import is healthy.
	seed(2 * time.Hour)
	rec = doJSON(t, h, http.MethodGet, "/health/data", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("with fresh data: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("expected status ok, got %v", body["status"])
	}

	// Past the window, the check must fail so the alert can fire. This is
	// the case the whole endpoint exists for: the service is healthy and
	// serving, but the data behind it stopped being refreshed.
	seed(10 * 24 * time.Hour)
	rec = doJSON(t, h, http.MethodGet, "/health/data", nil, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("with stale data: expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "stale" {
		t.Errorf("expected status stale, got %v", body["status"])
	}
}
