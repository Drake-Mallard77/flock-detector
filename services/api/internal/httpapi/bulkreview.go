package httpapi

import (
	"encoding/json"
	"net/http"

	"flockwatch/api/internal/models"
)

// maxBulkReview caps one request. Large enough to clear a derived-candidate
// queue in a few passes, small enough that a mistaken click can't rewrite
// the entire public record in one call — and small enough to stay a single
// fast statement.
const maxBulkReview = 200

type bulkReviewRequest struct {
	IDs    []string `json:"ids"`
	Status string   `json:"status"`
}

// handleBulkReviewDeployments sets the same status on many deployments at
// once, for clearing the OSM-derived candidate queue.
//
// Same authorization as the single-record path: moderator or admin, with
// the role re-read from the database per request. The reviewer is recorded
// on every row, so a bulk action is as attributable as an individual one —
// that matters more here, not less, because one click can move hundreds of
// records.
func (s *Server) handleBulkReviewDeployments(w http.ResponseWriter, r *http.Request) {
	var req bulkReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if len(req.IDs) > maxBulkReview {
		writeError(w, http.StatusBadRequest,
			"too many ids in one request; the maximum is 200")
		return
	}
	if !models.IsValidDeploymentStatus(req.Status) {
		writeError(w, http.StatusBadRequest,
			"invalid status, must be one of: "+joinDeploymentStatuses())
		return
	}

	c, _ := r.Context().Value(claimsContextKey).(*claims)
	var reviewerID *string
	if c != nil {
		reviewerID = &c.Subject
	}

	// One statement over the whole set: a partial failure that left some
	// records reviewed and others not would be worse than failing outright,
	// since the reviewer has no way to tell which took effect.
	tag, err := s.db.Exec(r.Context(), `
		UPDATE deployments
		SET status = $1, reviewed_by = $2, last_reviewed_at = now(), updated_at = now()
		WHERE id = ANY($3)
	`, req.Status, reviewerID, req.IDs)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not apply the bulk review", err)
		return
	}

	// Reports what actually changed rather than echoing the request: ids
	// that don't exist are silently skipped by the UPDATE, and the caller
	// should see that rather than assume all of them applied.
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  req.Status,
		"updated": tag.RowsAffected(),
	})
}
