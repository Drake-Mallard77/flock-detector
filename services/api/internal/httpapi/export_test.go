package httpapi

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestExportDeploymentsCSV(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	seedRecord(t, "Nashville Police Department", "Nashville", "TN", "osm_documented")
	seedRecord(t, "Atlanta Police Department", "Atlanta", "GA", "confirmed")
	// Unvetted: must not be handed out as part of the published dataset.
	seedRecord(t, "Somewhere Sheriff", "Somewhere", "MT", "under_review")

	rec := doJSON(t, h, http.MethodGet, "/export/deployments.csv", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("expected a CSV content type, got %q", ct)
	}
	// Without this browsers render the file instead of saving it.
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("expected an attachment disposition, got %q", cd)
	}
	// ODbL travels with the data; CSV has nowhere safe to put it in-band.
	if rec.Header().Get("X-License") != "ODbL-1.0" {
		t.Error("the licence should travel with the download")
	}

	records, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected a header plus 2 published records, got %d rows", len(records))
	}
	if records[0][0] != "agency_name" || records[0][12] != "url" {
		t.Errorf("unexpected header row: %v", records[0])
	}

	body := rec.Body.String()
	if strings.Contains(body, "Somewhere Sheriff") {
		t.Error("an under_review record leaked into the published export")
	}
	// The URL column is what makes a row traceable back to its sources.
	if !strings.Contains(body, "https://theflockwatcher.com/state/tn/") {
		t.Error("expected a canonical record URL in the export")
	}
}

func TestExportCamerasCSV(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	dep := seedRecord(t, "Detroit Police Department", "Detroit", "MI", "osm_documented")
	attachCameras(t, dep, 3, "export")

	rec := doJSON(t, h, http.MethodGet, "/export/cameras.csv", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	records, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("expected a header plus 3 cameras, got %d rows", len(records))
	}
	// Coordinates must be plain decimals: scientific notation is silently
	// mangled by spreadsheets back into a different location.
	for _, row := range records[1:] {
		if strings.ContainsAny(row[0]+row[1], "eE") {
			t.Errorf("coordinate in scientific notation: %v", row[:2])
		}
	}
}

func TestExportCamerasGeoJSON(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	dep := seedRecord(t, "Detroit Police Department", "Detroit", "MI", "osm_documented")
	attachCameras(t, dep, 2, "geo")
	// Give one a direction so the property is exercised.
	if _, err := testPool.Exec(context.Background(),
		`UPDATE camera_sightings SET direction = 270 WHERE external_id = 'osm:node:geo:0'`); err != nil {
		t.Fatalf("set direction: %v", err)
	}

	rec := doJSON(t, h, http.MethodGet, "/export/cameras.geojson", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Parsed rather than substring-matched: the collection is assembled by
	// hand-writing the wrapper and streaming features into it, so a missing
	// comma or bracket is exactly the bug worth catching, and it would not
	// show up in a contains() check.
	var fc struct {
		Type        string `json:"type"`
		Attribution string `json:"attribution"`
		Features    []struct {
			Type     string `json:"type"`
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatalf("output is not valid GeoJSON: %v", err)
	}

	if fc.Type != "FeatureCollection" {
		t.Errorf("expected a FeatureCollection, got %q", fc.Type)
	}
	if !strings.Contains(fc.Attribution, "OpenStreetMap") {
		t.Error("ODbL attribution must travel with the exported database")
	}
	if len(fc.Features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(fc.Features))
	}

	f := fc.Features[0]
	if f.Geometry.Type != "Point" || len(f.Geometry.Coordinates) != 2 {
		t.Fatalf("bad geometry: %+v", f.Geometry)
	}
	// GeoJSON is [lng, lat]. Swapping them puts Detroit in Somalia and
	// nothing about the file looks wrong until it's plotted.
	lng, lat := f.Geometry.Coordinates[0], f.Geometry.Coordinates[1]
	if lng > -80 || lng < -90 {
		t.Errorf("longitude out of range for the seeded point: %v", lng)
	}
	if lat < 30 || lat > 40 {
		t.Errorf("latitude out of range for the seeded point: %v", lat)
	}
}

// An empty result still has to be a well-formed document, or a consumer
// gets a parse error where it should get zero rows.
func TestExportCamerasGeoJSON_EmptyIsStillValid(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s.Router(), http.MethodGet, "/export/cameras.geojson", nil, "")

	var fc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
		t.Fatalf("empty export is not valid JSON: %v (%s)", err, rec.Body.String())
	}
	features, ok := fc["features"].([]any)
	if !ok || len(features) != 0 {
		t.Errorf("expected an empty feature list, got %v", fc["features"])
	}
}
