package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// handleGetDeploymentBySlug serves GET /deployments/by-slug/{state}/{slug}.
//
// The UUID route stays permanently. Links to it are already published — the
// sitemap Google has indexed is full of them — and a public-records project
// breaking its own citations to tidy up its URLs would be a poor trade. The
// client redirects UUID URLs to the readable form; the API serves both.
func (s *Server) handleGetDeploymentBySlug(w http.ResponseWriter, r *http.Request) {
	// Uppercased because the path carries the state lowercase (/state/tn)
	// while the column stores it uppercase.
	state := strings.ToUpper(chi.URLParam(r, "state"))
	slug := strings.ToLower(chi.URLParam(r, "slug"))

	row := s.db.QueryRow(r.Context(), `
		SELECT id, agency_name, slug, city, state, county,
		       ST_Y(location::geometry), ST_X(location::geometry),
		       documented_units, evidence_type, source_links, status, notes,
		       operator_type,
		       created_by, reviewed_by, last_reviewed_at, created_at, updated_at
		FROM deployments
		WHERE state = $1 AND slug = $2
	`, state, slug)

	d, err := scanDeployment(row)
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	writeJSON(w, http.StatusOK, d)
}
