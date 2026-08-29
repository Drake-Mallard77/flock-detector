package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func seedRecord(t *testing.T, agency, city, state, status string) string {
	t.Helper()
	var id string
	err := testPool.QueryRow(context.Background(), `
		INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links)
		VALUES ($1, $2, $3, $4, 'osm_import', ARRAY['https://example.test'])
		RETURNING id
	`, agency, city, state, status).Scan(&id)
	if err != nil {
		t.Fatalf("seed %q: %v", agency, err)
	}
	return id
}

func attachCameras(t *testing.T, deploymentID string, n int, tag string) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := testPool.Exec(context.Background(), `
			INSERT INTO camera_sightings (location, source, status, external_id, deployment_id)
			VALUES (ST_SetSRID(ST_MakePoint(-84.39, 33.75), 4326)::geography,
			        'osm_import', 'confirmed', $1, $2)
		`, fmt.Sprintf("osm:node:%s:%d", tag, i), deploymentID)
		if err != nil {
			t.Fatalf("attach camera: %v", err)
		}
	}
}

func TestListDuplicates_FindsPunctuationVariants(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")

	// The real-world case: a curly apostrophe and a straight one.
	a := seedRecord(t, "Nevada County Sheriff\u2019s Office", "Nevada County", "CA", "osm_documented")
	seedRecord(t, "Nevada County Sheriff's Office", "Nevada County", "CA", "under_review")
	// Same name, different state — not a duplicate.
	seedRecord(t, "Nevada County Sheriff's Office", "Nevada County", "NV", "osm_documented")
	// Unrelated.
	seedRecord(t, "Atlanta Police Department", "Atlanta", "GA", "confirmed")
	attachCameras(t, a, 3, "nevada")

	rec := doJSON(t, h, http.MethodGet, "/deployments/duplicates", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var groups []duplicateGroup
	if err := json.Unmarshal(rec.Body.Bytes(), &groups); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(groups) != 1 {
		t.Fatalf("expected exactly one duplicate group, got %d: %+v", len(groups), groups)
	}
	if groups[0].State != "CA" {
		t.Errorf("expected the CA group, got %s", groups[0].State)
	}
	if len(groups[0].Records) != 2 {
		t.Fatalf("expected 2 records in the group, got %d", len(groups[0].Records))
	}
	// Ordered by linked cameras first: the record cameras already point at
	// is the obvious survivor, and it should be the one offered first.
	if groups[0].Records[0].LinkedCameras != 3 {
		t.Errorf("expected the camera-bearing record first, got %+v", groups[0].Records[0])
	}
}

func TestMergeDeployment(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")

	survivor := seedRecord(t, "Detroit Police Department", "Detroit", "MI", "osm_documented")
	duplicate := seedRecord(t, "Detroit Police Dept.", "Detroit", "MI", "under_review")
	attachCameras(t, survivor, 2, "keep")
	attachCameras(t, duplicate, 4, "move")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+survivor+"/merge",
		map[string]string{"duplicate_id": duplicate}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["cameras_moved"] != float64(4) {
		t.Errorf("expected 4 cameras moved, got %v", out["cameras_moved"])
	}

	// Every camera now hangs off the survivor.
	var onSurvivor, onDuplicate int
	testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM camera_sightings WHERE deployment_id = $1`, survivor).Scan(&onSurvivor)
	testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM camera_sightings WHERE deployment_id = $1`, duplicate).Scan(&onDuplicate)
	if onSurvivor != 6 {
		t.Errorf("expected all 6 cameras on the survivor, got %d", onSurvivor)
	}
	if onDuplicate != 0 {
		t.Errorf("expected no cameras left on the duplicate, got %d", onDuplicate)
	}

	// The duplicate is retired, not deleted, and says where it went.
	var status string
	var notes *string
	testPool.QueryRow(context.Background(),
		`SELECT status, notes FROM deployments WHERE id = $1`, duplicate).Scan(&status, &notes)
	if status != "removed" {
		t.Errorf("expected the duplicate to be removed, got %q", status)
	}
	if notes == nil || !strings.Contains(*notes, "/state/mi/") {
		t.Errorf("the retired record should point at its survivor, got %v", notes)
	}
}

func TestMerge_RejectsSelfAndUnauthorised(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")

	id := seedRecord(t, "Somewhere PD", "Somewhere", "TX", "confirmed")

	if rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/merge",
		map[string]string{"duplicate_id": id}, token); rec.Code != http.StatusBadRequest {
		t.Errorf("merging a record into itself should be rejected, got %d", rec.Code)
	}

	// Merging rewrites the public record; it must not be open to anyone.
	other := seedRecord(t, "Somewhere Police Dept", "Somewhere", "TX", "under_review")
	if rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/merge",
		map[string]string{"duplicate_id": other}, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without a token, got %d", rec.Code)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle || len(needle) == 0 ||
			indexOf(haystack, needle) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
