package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"flockwatch/api/internal/models"
)

// handleListCameras returns camera sightings, optionally filtered to a
// bounding box via ?bbox=west,south,east,north. This is the opt-in precise
// pin layer and is not shown by default in the web UI.
func (s *Server) handleListCameras(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var west, south, east, north float64
	hasBBox := false
	if b := q.Get("bbox"); b != "" {
		var err error
		west, south, east, north, err = parseBBox(b)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid bbox, expected west,south,east,north")
			return
		}
		hasBBox = true
	}

	// Optional filters. Empty string means "no filter" for each, so a plain
	// /cameras call still returns everything.
	source := q.Get("source")
	if source != "" && source != "osm_import" && source != "user_submission" {
		writeError(w, http.StatusBadRequest, "invalid source, must be osm_import or user_submission")
		return
	}

	status := q.Get("status")
	if status != "" && status != "confirmed" && status != "under_review" && status != "removed" {
		writeError(w, http.StatusBadRequest, "invalid status, must be confirmed, under_review, or removed")
		return
	}

	// manufacturer=unknown selects rows where OSM didn't record one, which
	// can't be expressed as a plain equality match.
	manufacturer := q.Get("manufacturer")
	manufacturerUnknown := manufacturer == "unknown"
	if manufacturerUnknown {
		manufacturer = ""
	}

	// The bbox comparison runs in geometry space (location::geometry),
	// NOT geography. Casting the envelope to geography makes PostGIS
	// interpret a box wider than 180° as wrapping the *short* way around
	// the globe, so a zoomed-out viewport (e.g. -170,20,170,55) matched
	// zero rows and returned an empty map with HTTP 200 — no error to
	// notice. A map viewport is a planar rectangle in lon/lat, so a planar
	// comparison is both correct and what the GiST index supports.
	sql := `
		SELECT id, deployment_id, ST_Y(location::geometry), ST_X(location::geometry),
		       direction, camera_type, manufacturer, photo_url, source, status,
		       external_id, state, created_by, created_at
		FROM camera_sightings
		WHERE (
			NOT $1 OR location::geometry && ST_MakeEnvelope($2, $3, $4, $5, 4326)
		)
		  AND ($6 = '' OR source = $6)
		  AND ($7 = '' OR status = $7)
		  AND ($8 = '' OR manufacturer = $8)
		  AND (NOT $9 OR manufacturer IS NULL)
		ORDER BY created_at DESC
		LIMIT 1000
	`
	rows, err := s.db.Query(r.Context(), sql, hasBBox, west, south, east, north,
		source, status, manufacturer, manufacturerUnknown)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	cameras := []models.CameraSighting{}
	for rows.Next() {
		var c models.CameraSighting
		if err := rows.Scan(
			&c.ID, &c.DeploymentID, &c.Lat, &c.Lng,
			&c.Direction, &c.CameraType, &c.Manufacturer, &c.PhotoURL, &c.Source, &c.Status,
			&c.ExternalID, &c.State,
			&c.CreatedBy, &c.CreatedAt,
		); err != nil {
			writeError(w, http.StatusInternalServerError, "scan failed")
			return
		}
		cameras = append(cameras, c)
	}

	writeJSON(w, http.StatusOK, cameras)
}

// handleListManufacturers returns the manufacturers actually present in the
// data, with counts, most common first. The filter UI reads this rather than
// hardcoding a vendor list: OSM contributors add vendors over time, and a
// static list silently goes wrong (it listed 4 while the data held 20+).
func (s *Server) handleListManufacturers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		SELECT manufacturer, count(*)
		FROM camera_sightings
		WHERE manufacturer IS NOT NULL
		GROUP BY manufacturer
		ORDER BY count(*) DESC, manufacturer ASC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	type manufacturerCount struct {
		Manufacturer string `json:"manufacturer"`
		Count        int    `json:"count"`
	}

	out := []manufacturerCount{}
	for rows.Next() {
		var m manufacturerCount
		if err := rows.Scan(&m.Manufacturer, &m.Count); err != nil {
			writeError(w, http.StatusInternalServerError, "scan failed")
			return
		}
		out = append(out, m)
	}

	writeJSON(w, http.StatusOK, out)
}

type createCameraRequest struct {
	DeploymentID *string `json:"deployment_id"`
	Lat          float64 `json:"lat"`
	Lng          float64 `json:"lng"`
	Direction    *int    `json:"direction"`
	CameraType   *string `json:"camera_type"`
	PhotoURL     *string `json:"photo_url"`
}

func (s *Server) handleCreateCamera(w http.ResponseWriter, r *http.Request) {
	var req createCameraRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Lat == 0 && req.Lng == 0 {
		writeError(w, http.StatusBadRequest, "lat and lng are required")
		return
	}

	var id string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO camera_sightings (
			deployment_id, location, direction, camera_type, photo_url, source, status
		) VALUES (
			$1, ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, $5, $6,
			'user_submission', 'under_review'
		) RETURNING id
	`, req.DeploymentID, req.Lng, req.Lat, req.Direction, req.CameraType, req.PhotoURL).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "under_review"})
}

func parseBBox(s string) (west, south, east, north float64, err error) {
	parts := splitCSV(s)
	if len(parts) != 4 {
		return 0, 0, 0, 0, errBadBBox
	}
	vals := make([]float64, 4)
	for i, p := range parts {
		vals[i], err = strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, 0, 0, 0, errBadBBox
		}
	}
	return vals[0], vals[1], vals[2], vals[3], nil
}
