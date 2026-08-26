package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"flockwatch/api/internal/models"
)

func createCamera(t *testing.T, h http.Handler, lat, lng float64) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/cameras", map[string]any{
		"lat": lat, "lng": lng, "direction": 180, "camera_type": "Flock Falcon",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create camera: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp["id"]
}

func TestCreateCamera_Valid(t *testing.T) {
	s := newTestServer(t)
	id := createCamera(t, s.Router(), 39.799, -89.644)
	if id == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestListCameras_BBoxFilter(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	// Springfield, IL
	createCamera(t, h, 39.799, -89.644)

	rec := doJSON(t, h, http.MethodGet, "/cameras?bbox=-90,39,-89,40", nil, "")
	var inside []models.CameraSighting
	json.Unmarshal(rec.Body.Bytes(), &inside)
	if len(inside) != 1 {
		t.Fatalf("expected 1 camera inside the bbox, got %d", len(inside))
	}

	// A bbox nowhere near Illinois (middle of the Atlantic).
	rec = doJSON(t, h, http.MethodGet, "/cameras?bbox=-40,0,-30,10", nil, "")
	var outside []models.CameraSighting
	json.Unmarshal(rec.Body.Bytes(), &outside)
	if len(outside) != 0 {
		t.Fatalf("expected 0 cameras outside the bbox, got %d", len(outside))
	}
}

// Regression test: a viewport wider than 180° must still match. Casting the
// envelope to geography made PostGIS treat a wide box as wrapping the short
// way around the globe, so any zoomed-out map view silently returned zero
// results with HTTP 200. See handleListCameras.
func TestListCameras_WideBBoxStillMatches(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	createCamera(t, h, 39.799, -89.644) // Springfield, IL

	for _, bbox := range []string{
		"-125,24,-66,50",  // continental US
		"-170,20,170,55",  // 340° wide — the case that used to return 0
		"-180,-90,180,90", // whole world
	} {
		rec := doJSON(t, h, http.MethodGet, "/cameras?bbox="+bbox, nil, "")
		var cams []models.CameraSighting
		json.Unmarshal(rec.Body.Bytes(), &cams)
		if len(cams) != 1 {
			t.Errorf("bbox=%s: expected the camera to be found, got %d", bbox, len(cams))
		}
	}
}

func TestListCameras_Filters(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	// A user submission (source=user_submission, status=under_review, no
	// manufacturer) via the public endpoint...
	createCamera(t, h, 39.799, -89.644)

	// ...and two OSM-style rows inserted directly, since the public POST
	// endpoint deliberately can't set source/status/manufacturer.
	for _, m := range []struct {
		ext          string
		manufacturer any
	}{
		{"osm:node:1", "Flock Safety"},
		{"osm:node:2", nil}, // manufacturer unrecorded in OSM
	} {
		_, err := testPool.Exec(context.Background(), `
			INSERT INTO camera_sightings (location, source, status, external_id, manufacturer)
			VALUES (ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
			        'osm_import', 'confirmed', $3, $4)
		`, -89.65, 39.80, m.ext, m.manufacturer)
		if err != nil {
			t.Fatalf("seed %s: %v", m.ext, err)
		}
	}

	count := func(query string) int {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/cameras"+query, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", query, rec.Code, rec.Body.String())
		}
		var cams []models.CameraSighting
		json.Unmarshal(rec.Body.Bytes(), &cams)
		return len(cams)
	}

	if got := count(""); got != 3 {
		t.Errorf("no filter: expected 3, got %d", got)
	}
	if got := count("?source=osm_import"); got != 2 {
		t.Errorf("source=osm_import: expected 2, got %d", got)
	}
	if got := count("?source=user_submission"); got != 1 {
		t.Errorf("source=user_submission: expected 1, got %d", got)
	}
	if got := count("?status=confirmed"); got != 2 {
		t.Errorf("status=confirmed: expected 2, got %d", got)
	}
	if got := count("?manufacturer=Flock+Safety"); got != 1 {
		t.Errorf("manufacturer=Flock Safety: expected 1, got %d", got)
	}
	// "unknown" is a distinct concept from any specific manufacturer: it
	// means OSM recorded none, which an equality match can't express.
	if got := count("?manufacturer=unknown"); got != 2 {
		t.Errorf("manufacturer=unknown: expected 2 (the OSM row with no manufacturer + the user submission), got %d", got)
	}
	// Filters compose.
	if got := count("?source=osm_import&manufacturer=unknown"); got != 1 {
		t.Errorf("source+manufacturer: expected 1, got %d", got)
	}
}

func TestListManufacturers(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	// Two Flock, one Genetec, one with no manufacturer recorded.
	seed := []any{"Flock Safety", "Flock Safety", "Genetec", nil}
	for i, m := range seed {
		_, err := testPool.Exec(context.Background(), `
			INSERT INTO camera_sightings (location, source, status, external_id, manufacturer)
			VALUES (ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
			        'osm_import', 'confirmed', $3, $4)
		`, -89.65, 39.80, fmt.Sprintf("osm:node:%d", i), m)
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	rec := doJSON(t, h, http.MethodGet, "/cameras/manufacturers", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got []struct {
		Manufacturer string `json:"manufacturer"`
		Count        int    `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// NULL manufacturers are excluded — "not recorded" is offered by the UI
	// as a separate option, not as a vendor name.
	if len(got) != 2 {
		t.Fatalf("expected 2 manufacturers, got %d: %+v", len(got), got)
	}
	// Most common first.
	if got[0].Manufacturer != "Flock Safety" || got[0].Count != 2 {
		t.Errorf("expected Flock Safety with count 2 first, got %+v", got[0])
	}
	if got[1].Manufacturer != "Genetec" || got[1].Count != 1 {
		t.Errorf("expected Genetec with count 1 second, got %+v", got[1])
	}
}

func TestListCameras_InvalidFilters(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	for _, q := range []string{"?source=bogus", "?status=bogus"} {
		rec := doJSON(t, h, http.MethodGet, "/cameras"+q, nil, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", q, rec.Code)
		}
	}
}

func TestListCameras_InvalidBBox(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	// Note: no bbox param at all is valid (returns unfiltered results) —
	// see TestListCameras_NoBBoxReturnsAll. These are malformed non-empty
	// bbox values, which should 400.
	cases := []string{
		"not,a,valid,bbox",
		"1,2,3",     // too few parts
		"1,2,3,4,5", // too many parts
	}
	for _, bbox := range cases {
		rec := doJSON(t, h, http.MethodGet, "/cameras?bbox="+bbox, nil, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("bbox=%q: expected 400, got %d: %s", bbox, rec.Code, rec.Body.String())
		}
	}
}

func TestListCameras_NoBBoxReturnsAll(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	createCamera(t, h, 39.799, -89.644)

	rec := doJSON(t, h, http.MethodGet, "/cameras", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var cams []models.CameraSighting
	json.Unmarshal(rec.Body.Bytes(), &cams)
	if len(cams) != 1 {
		t.Fatalf("expected 1 camera with no bbox filter, got %d", len(cams))
	}
}
