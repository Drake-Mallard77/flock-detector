package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestDeploymentCameras(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	var depID string
	err := testPool.QueryRow(context.Background(), `
		INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links, documented_units)
		VALUES ('Springfield Police Department', 'Springfield', 'IL', 'osm_documented',
		        'osm_import', ARRAY['https://example.test'], 40)
		RETURNING id
	`).Scan(&depID)
	if err != nil {
		t.Fatalf("seed deployment: %v", err)
	}

	// Three linked to this record, one linked to nothing.
	for i := 0; i < 3; i++ {
		_, err := testPool.Exec(context.Background(), `
			INSERT INTO camera_sightings (location, source, status, external_id, deployment_id, direction)
			VALUES (ST_SetSRID(ST_MakePoint($1, 39.8), 4326)::geography,
			        'osm_import', 'confirmed', $2, $3, $4)
		`, -89.6+float64(i)*0.001, fmt.Sprintf("osm:node:linked:%d", i), depID, 90)
		if err != nil {
			t.Fatalf("seed linked camera: %v", err)
		}
	}
	_, err = testPool.Exec(context.Background(), `
		INSERT INTO camera_sightings (location, source, status, external_id)
		VALUES (ST_SetSRID(ST_MakePoint(-89.7, 39.9), 4326)::geography,
		        'osm_import', 'confirmed', 'osm:node:unlinked')
	`)
	if err != nil {
		t.Fatalf("seed unlinked camera: %v", err)
	}

	rec := doJSON(t, h, http.MethodGet, "/deployments/"+depID+"/cameras", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got deploymentCamerasResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Linked != 3 {
		t.Errorf("expected 3 linked cameras, got %d", got.Linked)
	}
	if len(got.Cameras) != 3 {
		t.Errorf("expected 3 camera points, got %d", len(got.Cameras))
	}
	for _, c := range got.Cameras {
		if c.Lat == 0 || c.Lng == 0 {
			t.Errorf("camera is missing coordinates: %+v", c)
		}
	}

	// A record with nothing linked must return an empty list, not an error.
	// This is the common case — 85% of cameras carry no operator tag — so it
	// has to be a normal, quiet answer rather than an exception.
	var emptyID string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links)
		VALUES ('Nowhere PD', 'Nowhere', 'MT', 'under_review', 'osm_import', ARRAY['https://example.test'])
		RETURNING id
	`).Scan(&emptyID); err != nil {
		t.Fatalf("seed empty deployment: %v", err)
	}
	rec = doJSON(t, h, http.MethodGet, "/deployments/"+emptyID+"/cameras", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for a record with no linked cameras, got %d", rec.Code)
	}
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Linked != 0 || len(got.Cameras) != 0 {
		t.Errorf("expected an empty list, got %+v", got)
	}
}

// A malformed id must not be reported the same way as a database failure.
func TestDeploymentCameras_MalformedID(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s.Router(), http.MethodGet, "/deployments/not-a-uuid/cameras", nil, "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for a malformed id, got %d", rec.Code)
	}
}
