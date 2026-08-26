package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"flockwatch/api/internal/models"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// handleListDeployments supports optional filters: state, status, city, and
// pagination via limit/offset. bbox filtering (west,south,east,north) is
// applied when all four are present and a location exists.
func (s *Server) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := defaultPageSize
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 && v <= maxPageSize {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v >= 0 {
		offset = v
	}

	// Free-text search across the fields a reader would actually type:
	// agency, city, or state. Done in SQL rather than filtering the fetched
	// page in the browser — a client-side filter only ever searches the
	// rows already loaded (at most one page), so anything past the first
	// page is invisible to it and the search silently appears broken.
	//
	// ILIKE with a wrapped wildcard is deliberate at this size: the table
	// holds agency-level records, not the 130k+ camera rows, so a trigram
	// or full-text index isn't earning its complexity yet. Revisit if this
	// grows past a few thousand rows.
	search := strings.TrimSpace(q.Get("q"))

	sql := `
		SELECT id, agency_name, city, state, county,
		       ST_Y(location::geometry), ST_X(location::geometry),
		       documented_units, evidence_type, source_links, status, notes,
		       operator_type,
		       created_by, reviewed_by, last_reviewed_at, created_at, updated_at
		FROM deployments
		WHERE ($1 = '' OR state = $1)
		  AND ($2 = '' OR status = $2)
		  AND ($3 = '' OR city = $3)
		  AND (
		    $6 = '' OR
		    agency_name ILIKE '%' || $6 || '%' OR
		    city        ILIKE '%' || $6 || '%' OR
		    state       ILIKE '%' || $6 || '%'
		  )
		ORDER BY updated_at DESC
		LIMIT $4 OFFSET $5
	`
	rows, err := s.db.Query(r.Context(), sql,
		q.Get("state"), q.Get("status"), q.Get("city"), limit, offset, search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	defer rows.Close()

	deployments := []models.Deployment{}
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "scan failed")
			return
		}
		deployments = append(deployments, d)
	}

	writeJSON(w, http.StatusOK, deployments)
}

func (s *Server) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	row := s.db.QueryRow(r.Context(), `
		SELECT id, agency_name, city, state, county,
		       ST_Y(location::geometry), ST_X(location::geometry),
		       documented_units, evidence_type, source_links, status, notes,
		       operator_type,
		       created_by, reviewed_by, last_reviewed_at, created_at, updated_at
		FROM deployments WHERE id = $1
	`, id)

	d, err := scanDeployment(row)
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	writeJSON(w, http.StatusOK, d)
}

type createDeploymentRequest struct {
	AgencyName      string   `json:"agency_name"`
	City            string   `json:"city"`
	State           string   `json:"state"`
	County          *string  `json:"county"`
	Lat             *float64 `json:"lat"`
	Lng             *float64 `json:"lng"`
	DocumentedUnits *int     `json:"documented_units"`
	EvidenceType    string   `json:"evidence_type"`
	SourceLinks     []string `json:"source_links"`
	Notes           *string  `json:"notes"`
}

// handleCreateDeployment accepts a public submission. It always lands in
// under_review status regardless of caller-supplied status; a moderator must
// promote it via POST /deployments/{id}/review.
func (s *Server) handleCreateDeployment(w http.ResponseWriter, r *http.Request) {
	var req createDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.AgencyName == "" || req.City == "" || req.State == "" || req.EvidenceType == "" {
		writeError(w, http.StatusBadRequest, "agency_name, city, state, and evidence_type are required")
		return
	}
	if !models.IsValidEvidenceType(req.EvidenceType) {
		writeError(w, http.StatusBadRequest, "invalid evidence_type, must be one of: "+joinEvidenceTypes())
		return
	}
	if req.SourceLinks == nil {
		req.SourceLinks = []string{}
	}

	var id string
	err := s.db.QueryRow(r.Context(), `
		INSERT INTO deployments (
			agency_name, city, state, county, location,
			documented_units, evidence_type, source_links, status, notes
		) VALUES (
			$1, $2, $3, $4,
			CASE WHEN $5::float8 IS NULL OR $6::float8 IS NULL
			     THEN NULL ELSE ST_SetSRID(ST_MakePoint($6, $5), 4326)::geography END,
			$7, $8, $9, 'under_review', $10
		) RETURNING id
	`, req.AgencyName, req.City, req.State, req.County, req.Lat, req.Lng,
		req.DocumentedUnits, req.EvidenceType, req.SourceLinks, req.Notes).Scan(&id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "under_review"})
}

type reviewDeploymentRequest struct {
	Status string `json:"status"`
}

// handleReviewDeployment lets a moderator/admin set the reviewed status of a
// submission — the Review Desk action from the FlockWatch prototype.
func (s *Server) handleReviewDeployment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req reviewDeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if !models.IsValidDeploymentStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "invalid status, must be one of: "+joinDeploymentStatuses())
		return
	}

	c, _ := r.Context().Value(claimsContextKey).(*claims)
	var reviewerID *string
	if c != nil {
		reviewerID = &c.Subject
	}

	tag, err := s.db.Exec(r.Context(), `
		UPDATE deployments
		SET status = $1, reviewed_by = $2, last_reviewed_at = now(), updated_at = now()
		WHERE id = $3
	`, req.Status, reviewerID, id)
	if err != nil || tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": id, "status": req.Status})
}

// row is satisfied by both pgx.Row (QueryRow) and pgx.Rows (Query, via Next).
type row interface {
	Scan(dest ...interface{}) error
}

func scanDeployment(r row) (models.Deployment, error) {
	var d models.Deployment
	err := r.Scan(
		&d.ID, &d.AgencyName, &d.City, &d.State, &d.County,
		&d.Lat, &d.Lng,
		&d.DocumentedUnits, &d.EvidenceType, &d.SourceLinks, &d.Status, &d.Notes,
		&d.OperatorType,
		&d.CreatedBy, &d.ReviewedBy, &d.LastReviewedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	return d, err
}
