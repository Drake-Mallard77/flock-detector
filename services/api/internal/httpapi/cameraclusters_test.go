package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"testing"
)

// seedCounter keeps external_id unique across calls. Deriving it from the
// loop index alone collided the moment a test seeded two groups with the
// same manufacturer, which is the normal case.
var seedCounter int

func seedCameras(t *testing.T, points [][2]float64, manufacturer string) {
	t.Helper()
	for _, p := range points {
		seedCounter++
		_, err := testPool.Exec(context.Background(), `
			INSERT INTO camera_sightings (location, source, status, manufacturer, external_id)
			VALUES (ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography,
			        'osm_import', 'confirmed', $3, $4)
		`, p[1], p[0], manufacturer, fmt.Sprintf("osm:node:test:%d", seedCounter))
		if err != nil {
			t.Fatalf("seed camera: %v", err)
		}
	}
}

func getClusters(t *testing.T, h http.Handler, query string) cameraClustersResponse {
	t.Helper()
	rec := doJSON(t, h, http.MethodGet, "/cameras/clusters?"+query, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out cameraClustersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out
}

// The bug this endpoint exists to fix: every camera must be represented in
// the aggregate, however many there are. /cameras caps its response, so at
// low zoom the total it implied was simply wrong.
func TestCameraClusters_TotalCountsEveryCamera(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	// Two tight groups far apart, plus one isolated point.
	var atlanta, seattle [][2]float64
	for i := 0; i < 40; i++ {
		atlanta = append(atlanta, [2]float64{33.75 + float64(i)*0.0001, -84.39})
	}
	for i := 0; i < 25; i++ {
		seattle = append(seattle, [2]float64{47.60 + float64(i)*0.0001, -122.33})
	}
	seedCameras(t, atlanta, "Flock Safety")
	seedCameras(t, seattle, "Flock Safety")
	seedCameras(t, [][2]float64{{39.74, -104.99}}, "Motorola")

	got := getClusters(t, h, "bbox=-125,24,-66,50&zoom=4")
	if got.Total != 66 {
		t.Errorf("total should count all 66 seeded cameras, got %d", got.Total)
	}

	sum := 0
	for _, c := range got.Clusters {
		sum += c.Count
	}
	if sum != got.Total {
		t.Errorf("cluster counts sum to %d but total says %d", sum, got.Total)
	}
	// At national zoom the three sites are thousands of km apart, so they
	// must not be merged into one bubble.
	if len(got.Clusters) != 3 {
		t.Errorf("expected 3 separate clusters, got %d", len(got.Clusters))
	}
}

// A bubble should sit where its cameras are. Using the grid cell's centre
// instead puts it wherever the arbitrary grid happens to fall, which on a
// coastal cell can be out at sea.
func TestCameraClusters_PositionedOnActualPoints(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	seedCameras(t, [][2]float64{
		{33.7500, -84.3900},
		{33.7502, -84.3902},
		{33.7504, -84.3904},
	}, "Flock Safety")

	got := getClusters(t, h, "bbox=-125,24,-66,50&zoom=4")
	if len(got.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(got.Clusters))
	}
	c := got.Clusters[0]
	if math.Abs(c.Lat-33.7502) > 0.001 || math.Abs(c.Lng-(-84.3902)) > 0.001 {
		t.Errorf("cluster should sit on the mean of its points, got %.4f,%.4f", c.Lat, c.Lng)
	}
}

// Zooming in must split clusters apart, otherwise the map never resolves
// into individual sites.
func TestCameraClusters_SplitAsZoomIncreases(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	// Two groups ~50km apart: one bubble when zoomed out, two when in.
	seedCameras(t, [][2]float64{{33.75, -84.39}, {33.751, -84.391}}, "A")
	seedCameras(t, [][2]float64{{34.20, -84.39}, {34.201, -84.391}}, "B")

	wide := getClusters(t, h, "bbox=-125,24,-66,50&zoom=4")
	if len(wide.Clusters) != 1 {
		t.Errorf("at zoom 4 the two sites should merge, got %d clusters", len(wide.Clusters))
	}
	close := getClusters(t, h, "bbox=-85,33,-84,35&zoom=11")
	if len(close.Clusters) != 2 {
		t.Errorf("at zoom 11 they should separate, got %d clusters", len(close.Clusters))
	}
	if wide.Total != close.Total {
		t.Errorf("zoom changes grouping, not totals: %d vs %d", wide.Total, close.Total)
	}
}

// Filters have to mean the same thing here as on /cameras. If they drift, a
// bubble's count stops matching the points you see when you zoom into it.
func TestCameraClusters_RespectsFilters(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	seedCameras(t, [][2]float64{{33.75, -84.39}, {33.76, -84.40}}, "Flock Safety")
	seedCameras(t, [][2]float64{{33.77, -84.41}}, "Motorola")

	all := getClusters(t, h, "bbox=-125,24,-66,50&zoom=4")
	if all.Total != 3 {
		t.Errorf("unfiltered total should be 3, got %d", all.Total)
	}
	flock := getClusters(t, h, "bbox=-125,24,-66,50&zoom=4&manufacturer=Flock+Safety")
	if flock.Total != 2 {
		t.Errorf("filtered to Flock Safety should be 2, got %d", flock.Total)
	}
}

// A bad zoom should still render a map rather than an error page.
func TestCameraClusters_ClampsAbsurdZoom(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	seedCameras(t, [][2]float64{{33.75, -84.39}}, "Flock Safety")

	for _, z := range []string{"-40", "999", "not-a-number", ""} {
		got := getClusters(t, h, "bbox=-125,24,-66,50&zoom="+z)
		if got.Total != 1 {
			t.Errorf("zoom=%q: expected the camera to still be counted, got %d", z, got.Total)
		}
		if got.CellSize <= 0 {
			t.Errorf("zoom=%q: cell size must stay positive, got %v", z, got.CellSize)
		}
	}
}
