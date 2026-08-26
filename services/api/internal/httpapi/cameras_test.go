package httpapi

import (
	"encoding/json"
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
