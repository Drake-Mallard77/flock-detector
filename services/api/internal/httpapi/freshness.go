package httpapi

import (
	"net/http"
	"time"
)

// maxDataAge is how stale the camera data may get before something is
// wrong. The refresh runs weekly, so this allows a full cycle plus a
// missed run before complaining.
const maxDataAge = 9 * 24 * time.Hour

// handleDataFreshness reports whether the imported data is still current,
// returning 503 when it isn't so an uptime check can alert on it.
//
// This measures the outcome rather than the mechanism. The obvious
// alternative — alerting when the scheduled job stops running — can't
// actually be expressed: Cloud Monitoring caps metric-absence conditions at
// 23h30m, and this job runs weekly. More importantly, "a job ran" is a
// proxy. A job can run, log success, and still import nothing. What matters
// to a reader is whether the records are current, which is what this checks.
func (s *Server) handleDataFreshness(w http.ResponseWriter, r *http.Request) {
	var newest *time.Time
	err := s.db.QueryRow(r.Context(),
		`SELECT max(updated_at) FROM camera_sightings WHERE source = 'osm_import'`,
	).Scan(&newest)
	if err != nil {
		serverError(w, r, http.StatusServiceUnavailable, "could not determine data freshness", err)
		return
	}
	if newest == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "no data",
			"detail": "no imported camera records exist",
		})
		return
	}

	age := time.Since(*newest)
	body := map[string]any{
		"last_import":   newest.UTC().Format(time.RFC3339),
		"age_hours":     int(age.Hours()),
		"max_age_hours": int(maxDataAge.Hours()),
	}

	if age > maxDataAge {
		body["status"] = "stale"
		body["detail"] = "the weekly OpenStreetMap refresh has not completed recently"
		writeJSON(w, http.StatusServiceUnavailable, body)
		return
	}

	body["status"] = "ok"
	writeJSON(w, http.StatusOK, body)
}
