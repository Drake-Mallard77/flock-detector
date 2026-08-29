package httpapi

import (
	"net/http"
)

// StateStat is one row of the state index.
type StateStat struct {
	State string `json:"state"`
	// Deployments counts published agency records only — the same set the
	// sitemap promotes. Counting under_review here would advertise unvetted
	// submissions as documented findings on a page framed as a summary.
	Deployments int `json:"deployments"`
	// Cameras counts every imported camera in the state, published or not:
	// a camera location is an OpenStreetMap observation, not a claim this
	// project has reviewed, so there is no review status to filter on.
	Cameras int `json:"cameras"`
}

// handleStateStats serves GET /stats/states.
//
// Exists so the atlas can offer a way in other than a search box. With 1,150
// agency records across 50 states, the only route to a specific place was
// knowing its name and typing it — which fails the reader who wants to know
// what is deployed near them and has no agency name to search for.
func (s *Server) handleStateStats(w http.ResponseWriter, r *http.Request) {
	// One query rather than two and a merge in Go: the counts come from
	// different tables with different filters, and a FULL OUTER JOIN keeps
	// states that have cameras but no published record yet (and vice versa)
	// instead of silently dropping them.
	rows, err := s.db.Query(r.Context(), `
		WITH dep AS (
			SELECT state, count(*)::int AS n
			FROM deployments
			WHERE state IS NOT NULL AND status = ANY($1)
			GROUP BY state
		),
		cam AS (
			SELECT state, count(*)::int AS n
			FROM camera_sightings
			WHERE state IS NOT NULL
			GROUP BY state
		)
		SELECT coalesce(dep.state, cam.state) AS state,
		       coalesce(dep.n, 0),
		       coalesce(cam.n, 0)
		FROM dep
		FULL OUTER JOIN cam ON dep.state = cam.state
		ORDER BY state
	`, publishedStatuses)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load state totals", err)
		return
	}
	defer rows.Close()

	out := []StateStat{}
	for rows.Next() {
		var s StateStat
		if err := rows.Scan(&s.State, &s.Deployments, &s.Cameras); err != nil {
			serverError(w, r, http.StatusInternalServerError, "could not load state totals", err)
			return
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load state totals", err)
		return
	}

	// Changes only when the weekly import runs, and is requested on every
	// visit to the index.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, http.StatusOK, out)
}
