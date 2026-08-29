package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestStateStats(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	seedDeployment := func(state, status string) {
		t.Helper()
		_, err := testPool.Exec(context.Background(), `
			INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links)
			VALUES ($1, 'Somewhere', $2, $3, 'osm_import', ARRAY['https://example.test'])
		`, "Agency "+state+status, state, status)
		if err != nil {
			t.Fatalf("seed deployment: %v", err)
		}
	}

	seedDeployment("TN", "confirmed")
	seedDeployment("TN", "osm_documented")
	// Not published: must not be counted on a page framed as a summary of
	// what's documented.
	seedDeployment("TN", "under_review")
	seedDeployment("CA", "contract_found")

	for i, st := range []string{"TN", "TN", "TN", "CA"} {
		_, err := testPool.Exec(context.Background(), `
			INSERT INTO camera_sightings (location, source, status, state, external_id)
			VALUES (ST_SetSRID(ST_MakePoint(-86.7, 36.1), 4326)::geography,
			        'osm_import', 'confirmed', $1, $2)
		`, st, "osm:node:stat:"+string(rune('a'+i)))
		if err != nil {
			t.Fatalf("seed camera: %v", err)
		}
	}
	// A state with cameras but no published record at all — must still
	// appear, or the index silently omits places the data covers.
	_, err := testPool.Exec(context.Background(), `
		INSERT INTO camera_sightings (location, source, status, state, external_id)
		VALUES (ST_SetSRID(ST_MakePoint(-110.0, 43.0), 4326)::geography,
		        'osm_import', 'confirmed', 'WY', 'osm:node:stat:wy')
	`)
	if err != nil {
		t.Fatalf("seed WY camera: %v", err)
	}

	rec := doJSON(t, h, http.MethodGet, "/stats/states", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got []StateStat
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	by := map[string]StateStat{}
	for _, s := range got {
		by[s.State] = s
	}

	if by["TN"].Deployments != 2 {
		t.Errorf("TN should count 2 published records (not the under_review one), got %d", by["TN"].Deployments)
	}
	if by["TN"].Cameras != 3 {
		t.Errorf("TN should count 3 cameras, got %d", by["TN"].Cameras)
	}
	if by["CA"].Deployments != 1 || by["CA"].Cameras != 1 {
		t.Errorf("CA wrong: %+v", by["CA"])
	}
	wy, ok := by["WY"]
	if !ok {
		t.Fatal("a state with cameras but no published record must still appear")
	}
	if wy.Deployments != 0 || wy.Cameras != 1 {
		t.Errorf("WY should be 0 records / 1 camera, got %+v", wy)
	}
}
