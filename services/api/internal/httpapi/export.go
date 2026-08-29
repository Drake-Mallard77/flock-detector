package httpapi

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Bulk downloads of the published dataset.
//
// The atlas is built on ODbL data and exists to make surveillance
// procurement legible; a researcher or journalist wanting to analyse it
// should not have to scrape the map to do so. Serving the file directly is
// both more useful to them and cheaper for us than being scraped.
//
// Everything here streams. The camera table is 98MB on disk across 136,008
// rows; building that response in memory on a 512Mi container is how you
// get an OOM kill mid-request, which surfaces as a truncated download with
// nothing logged — the process dies before it can write an error.

// ODbL requires attribution to travel with a redistributed database.
// GeoJSON carries it as a top-level member. CSV has no comment convention
// every parser tolerates, so there it travels in a response header and is
// stated on the download page instead of being smuggled into row one where
// it would corrupt the parse.
const odblAttribution = "Camera locations © OpenStreetMap contributors, available under the Open Database License (ODbL). Agency records compiled by FlockWatch (theflockwatcher.com)."

// setExportHeaders marks the response as a download rather than something
// to render, and carries the licence.
func setExportHeaders(w http.ResponseWriter, contentType, filename string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("X-License", "ODbL-1.0")
	w.Header().Set("X-Attribution", odblAttribution)
	// The dataset only moves when the weekly import runs.
	w.Header().Set("Cache-Control", "public, max-age=3600")
}

func flushIfPossible(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// handleExportDeploymentsCSV serves GET /export/deployments.csv.
//
// Published records only — the same set the sitemap and the state pages
// use. Exporting unvetted candidates would hand out a file of unverified
// claims about named agencies, stripped of the context the site gives them.
func (s *Server) handleExportDeploymentsCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		SELECT agency_name, city, state, county,
		       ST_Y(location::geometry), ST_X(location::geometry),
		       documented_units, evidence_type, status,
		       array_to_string(source_links, ' | '),
		       last_reviewed_at, updated_at,
		       'https://theflockwatcher.com/state/' || lower(state) || '/' || slug
		FROM deployments
		WHERE status = ANY($1)
		ORDER BY state, city, agency_name
	`, publishedStatuses)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not build the export", err)
		return
	}
	defer rows.Close()

	setExportHeaders(w, "text/csv; charset=utf-8", "flockwatch-deployments.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"agency_name", "city", "state", "county", "lat", "lng",
		"documented_units", "evidence_type", "status", "source_links",
		"last_reviewed_at", "updated_at", "url",
	})

	for rows.Next() {
		var agency, city, state, evidence, status, sources, url string
		var county *string
		var lat, lng *float64
		var units *int
		var reviewed *time.Time
		var updated time.Time
		if err := rows.Scan(&agency, &city, &state, &county, &lat, &lng,
			&units, &evidence, &status, &sources, &reviewed, &updated, &url); err != nil {
			// The response is already committed with a 200, so this can only
			// be logged. A short file beats a wrong one, and the log says why.
			logEncodeFailure(r, err)
			break
		}
		_ = cw.Write([]string{
			agency, city, state, deref(county),
			floatOrEmpty(lat), floatOrEmpty(lng),
			intOrEmpty(units), evidence, status, sources,
			timeOrEmpty(reviewed), updated.UTC().Format(time.RFC3339), url,
		})
	}
	cw.Flush()
}

// rowsPerFlush balances latency against syscalls. csv.Writer buffers, so
// without periodic flushing the whole file accumulates before the first
// byte reaches the client — which both defeats streaming and puts the
// entire response in memory anyway, the thing this endpoint avoids.
const rowsPerFlush = 2000

// handleExportCamerasCSV serves GET /export/cameras.csv.
func (s *Server) handleExportCamerasCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		SELECT ST_Y(location::geometry), ST_X(location::geometry),
		       direction, camera_type, manufacturer, operator, state,
		       source, external_id, updated_at
		FROM camera_sightings
		ORDER BY state, external_id
	`)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not build the export", err)
		return
	}
	defer rows.Close()

	setExportHeaders(w, "text/csv; charset=utf-8", "flockwatch-cameras.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"lat", "lng", "direction", "camera_type", "manufacturer",
		"operator", "state", "source", "external_id", "updated_at",
	})

	n := 0
	for rows.Next() {
		var lat, lng float64
		var direction *int
		var camType, manufacturer, operator, state, externalID *string
		var source string
		var updated time.Time
		if err := rows.Scan(&lat, &lng, &direction, &camType, &manufacturer,
			&operator, &state, &source, &externalID, &updated); err != nil {
			logEncodeFailure(r, err)
			break
		}
		_ = cw.Write([]string{
			// 7 decimal places is ~1cm, past anything OSM claims, and keeps
			// the value from arriving in scientific notation where a
			// spreadsheet would mangle it back into a wrong coordinate.
			strconv.FormatFloat(lat, 'f', 7, 64),
			strconv.FormatFloat(lng, 'f', 7, 64),
			intOrEmpty(direction), deref(camType), deref(manufacturer),
			deref(operator), deref(state), source, deref(externalID),
			updated.UTC().Format(time.RFC3339),
		})
		if n++; n%rowsPerFlush == 0 {
			cw.Flush()
			flushIfPossible(w)
		}
	}
	cw.Flush()
}

// handleExportCamerasGeoJSON serves GET /export/cameras.geojson.
//
// The FeatureCollection is written by hand rather than marshalled from a
// slice: encoding/json needs the whole structure in memory before it emits
// anything, which for 136,000 features is precisely the allocation this
// endpoint exists to avoid. Each feature is marshalled on its own and
// streamed out.
func (s *Server) handleExportCamerasGeoJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		SELECT ST_Y(location::geometry), ST_X(location::geometry),
		       direction, camera_type, manufacturer, operator, state, source
		FROM camera_sightings
		ORDER BY state, external_id
	`)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not build the export", err)
		return
	}
	defer rows.Close()

	setExportHeaders(w, "application/geo+json; charset=utf-8", "flockwatch-cameras.geojson")

	fmt.Fprintf(w, `{"type":"FeatureCollection","attribution":%s,"features":[`,
		mustJSON(odblAttribution))

	first := true
	n := 0
	for rows.Next() {
		var lat, lng float64
		var direction *int
		var camType, manufacturer, operator, state *string
		var source string
		if err := rows.Scan(&lat, &lng, &direction, &camType, &manufacturer,
			&operator, &state, &source); err != nil {
			logEncodeFailure(r, err)
			break
		}

		props := map[string]any{"source": source}
		if direction != nil {
			props["direction"] = *direction
		}
		putIfSet(props, "camera_type", camType)
		putIfSet(props, "manufacturer", manufacturer)
		putIfSet(props, "operator", operator)
		putIfSet(props, "state", state)

		feature := map[string]any{
			"type": "Feature",
			// GeoJSON orders coordinates [lng, lat] — the reverse of every
			// other coordinate pair in this codebase.
			"geometry":   map[string]any{"type": "Point", "coordinates": []float64{lng, lat}},
			"properties": props,
		}
		encoded, err := json.Marshal(feature)
		if err != nil {
			logEncodeFailure(r, err)
			break
		}
		if !first {
			_, _ = w.Write([]byte(","))
		}
		first = false
		_, _ = w.Write(encoded)

		if n++; n%rowsPerFlush == 0 {
			flushIfPossible(w)
		}
	}

	_, _ = w.Write([]byte("]}"))
	flushIfPossible(w)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intOrEmpty(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func floatOrEmpty(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', 7, 64)
}

func timeOrEmpty(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func putIfSet(m map[string]any, key string, v *string) {
	if v != nil && *v != "" {
		m[key] = *v
	}
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `""`
	}
	return string(b)
}
