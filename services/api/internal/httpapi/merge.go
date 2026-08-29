package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Duplicate records exist because the derive job's identity check was
// case-insensitive but not punctuation-insensitive: OpenStreetMap carries
// both "Sheriff's Office" and "Sheriff’s Office", so a weekly run kept
// proposing agencies already in the atlas. The check is fixed, but the
// records it already created are still here, and on a public-records site
// two entries for one agency is a credibility problem rather than a
// cosmetic one.
//
// Merging is deliberately a moderator action rather than a migration.
// Choosing which of a pair survives — and therefore which agency name and
// which evidence the atlas stands behind — is a judgement about the record,
// not a mechanical cleanup.

type duplicateRecord struct {
	ID              string `json:"id"`
	AgencyName      string `json:"agency_name"`
	City            string `json:"city"`
	State           string `json:"state"`
	Status          string `json:"status"`
	Slug            string `json:"slug"`
	DocumentedUnits *int   `json:"documented_units,omitempty"`
	// LinkedCameras is the strongest signal for which record to keep: the
	// one cameras are already attached to needs no relinking.
	LinkedCameras int    `json:"linked_cameras"`
	CreatedAt     string `json:"created_at"`
}

type duplicateGroup struct {
	State   string            `json:"state"`
	Records []duplicateRecord `json:"records"`
}

// handleListDuplicates serves GET /deployments/duplicates (moderator only).
//
// Groups by the same normalised key the derive job now uses to decide
// identity, so what's listed here is exactly what would have been treated
// as one record had the check always been right.
func (s *Server) handleListDuplicates(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		WITH normalised AS (
			SELECT d.id, d.agency_name, d.city, d.state, d.status, d.slug,
			       d.documented_units, d.created_at,
			       regexp_replace(lower(d.agency_name), '[^a-z0-9]+', '', 'g') AS key,
			       (SELECT count(*) FROM camera_sightings c WHERE c.deployment_id = d.id) AS linked
			FROM deployments d
			WHERE d.state IS NOT NULL AND d.status <> 'removed'
		),
		dupes AS (
			SELECT state, key FROM normalised
			GROUP BY state, key HAVING count(*) > 1
		)
		SELECT n.state, n.key, n.id, n.agency_name, n.city, n.status, n.slug,
		       n.documented_units, n.linked, n.created_at
		FROM normalised n
		JOIN dupes ON dupes.state = n.state AND dupes.key = n.key
		ORDER BY n.state, n.key, n.linked DESC, n.created_at
	`)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load duplicates", err)
		return
	}
	defer rows.Close()

	// Grouped in Go rather than with a JSON aggregate in SQL: the shape is
	// small, and a nested aggregate here would be harder to read than the
	// loop it replaces.
	groups := []duplicateGroup{}
	var currentState, currentKey string
	for rows.Next() {
		var state, key string
		var created time.Time
		var rec duplicateRecord
		// The grouping key comes from SQL rather than being recomputed here,
		// so the rows and the groups can never disagree about what counts as
		// the same agency.
		if err := rows.Scan(&state, &key, &rec.ID, &rec.AgencyName, &rec.City,
			&rec.Status, &rec.Slug, &rec.DocumentedUnits, &rec.LinkedCameras, &created); err != nil {
			serverError(w, r, http.StatusInternalServerError, "could not load duplicates", err)
			return
		}
		rec.State = state
		rec.CreatedAt = created.UTC().Format(time.RFC3339)

		if len(groups) == 0 || currentState != state || currentKey != key {
			groups = append(groups, duplicateGroup{State: state})
			currentState, currentKey = state, key
		}
		groups[len(groups)-1].Records = append(groups[len(groups)-1].Records, rec)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load duplicates", err)
		return
	}

	writeJSON(w, http.StatusOK, groups)
}

type mergeRequest struct {
	// DuplicateID is the record being folded in. The record in the path is
	// the survivor — the URL names what you keep, not what you discard.
	DuplicateID string `json:"duplicate_id"`
}

// handleMergeDeployment serves POST /deployments/{id}/merge (moderator only).
//
// Nothing is deleted. The duplicate is marked 'removed' and keeps a note
// naming the record it was merged into, so the decision stays auditable —
// on a project whose claim is that every record is traceable, a silent
// DELETE would be the wrong tool even for a genuine duplicate.
func (s *Server) handleMergeDeployment(w http.ResponseWriter, r *http.Request) {
	survivorID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	var req mergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	duplicateID, err := uuid.Parse(req.DuplicateID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "duplicate_id must be a record id")
		return
	}
	if survivorID == duplicateID {
		writeError(w, http.StatusBadRequest, "a record cannot be merged into itself")
		return
	}

	c, _ := r.Context().Value(claimsContextKey).(*claims)
	var reviewerID *string
	if c != nil {
		reviewerID = &c.Subject
	}

	// One transaction: moving the cameras and retiring the duplicate have to
	// happen together. Half of this — cameras moved, duplicate still live,
	// or a retired record still holding the only link to its cameras — is
	// worse than neither.
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not merge these records", err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var moved int64
	tag, err := tx.Exec(r.Context(), `
		UPDATE camera_sightings
		SET deployment_id = $1, updated_at = now()
		WHERE deployment_id = $2
	`, survivorID, duplicateID)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not merge these records", err)
		return
	}
	moved = tag.RowsAffected()

	var survivorSlug, survivorState string
	err = tx.QueryRow(r.Context(),
		`SELECT slug, state FROM deployments WHERE id = $1`, survivorID,
	).Scan(&survivorSlug, &survivorState)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not merge these records", err)
		return
	}

	retire, err := tx.Exec(r.Context(), `
		UPDATE deployments
		SET status = 'removed',
		    reviewed_by = $2,
		    last_reviewed_at = now(),
		    updated_at = now(),
		    notes = coalesce(notes || E'\n\n', '') ||
		            'Merged into /state/' || lower($3) || '/' || $4 ||
		            ' as a duplicate record.'
		WHERE id = $1
	`, duplicateID, reviewerID, survivorState, survivorSlug)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not merge these records", err)
		return
	}
	if retire.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	// Same transaction as the retirement: a merged-away record whose history
	// does not say where it went is the one case where the trail matters most.
	note := "Merged into /state/" + strings.ToLower(survivorState) + "/" + survivorSlug
	if err := recordEvent(r.Context(), tx, duplicateID.String(), "merged",
		"", "removed", "", nil, &note, reviewerID); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not merge these records", err)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not merge these records", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"survivor_id":   survivorID.String(),
		"duplicate_id":  duplicateID.String(),
		"cameras_moved": moved,
	})
}
