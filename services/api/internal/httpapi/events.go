package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// execer is the slice of pgxpool.Pool and pgx.Tx that recordEvent needs, so
// it can run either standalone or inside a caller's transaction.
type execer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// Recording and reading what changed.
//
// Two different questions get answered here. "What has been decided about
// this record" is the audit trail — every status change, who made it and on
// what evidence. "What is new in the atlas" is the activity feed, which is
// what gives a reader a reason to come back.

// recordEvent appends to the audit trail.
//
// Takes a querier rather than the pool so it can run inside the caller's
// transaction: a merge that moved cameras and retired a record should not
// be able to commit without its event, or the trail acquires holes exactly
// where the interesting decisions are.
//
// Failures are returned, not swallowed. An untraceable decision on a
// public-records site is worse than a failed one, so the caller aborts.
func recordEvent(
	ctx context.Context,
	q execer,
	deploymentID, kind, fromStatus, toStatus string,
	evidenceType string,
	sourceLinks []string,
	note *string,
	actorID *string,
) error {
	var from *string
	if fromStatus != "" {
		from = &fromStatus
	}
	var evidence *string
	if evidenceType != "" {
		evidence = &evidenceType
	}
	if sourceLinks == nil {
		sourceLinks = []string{}
	}

	_, err := q.Exec(ctx, `
		INSERT INTO deployment_events (
			deployment_id, kind, from_status, to_status,
			evidence_type, source_links, note, actor_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, deploymentID, kind, from, toStatus, evidence, sourceLinks, note, actorID)
	return err
}

type deploymentEvent struct {
	Kind         string    `json:"kind"`
	FromStatus   *string   `json:"from_status,omitempty"`
	ToStatus     string    `json:"to_status"`
	EvidenceType *string   `json:"evidence_type,omitempty"`
	SourceLinks  []string  `json:"source_links"`
	Note         *string   `json:"note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`

	// Present on the site-wide feed so an entry can name and link its
	// record; omitted on a single record's own history, where it would
	// repeat on every row.
	AgencyName *string `json:"agency_name,omitempty"`
	State      *string `json:"state,omitempty"`
	Slug       *string `json:"slug,omitempty"`
}

// handleDeploymentEvents serves GET /deployments/{id}/events.
//
// Public. The whole point of an audit trail on a transparency project is
// that readers can see it — a decision log only moderators can read is
// documentation, not accountability.
func (s *Server) handleDeploymentEvents(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	rows, err := s.db.Query(r.Context(), `
		SELECT kind, from_status, to_status, evidence_type, source_links, note, created_at
		FROM deployment_events
		WHERE deployment_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, id)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load this record's history", err)
		return
	}
	defer rows.Close()

	out := []deploymentEvent{}
	for rows.Next() {
		var e deploymentEvent
		if err := rows.Scan(&e.Kind, &e.FromStatus, &e.ToStatus,
			&e.EvidenceType, &e.SourceLinks, &e.Note, &e.CreatedAt); err != nil {
			serverError(w, r, http.StatusInternalServerError, "could not load this record's history", err)
			return
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load this record's history", err)
		return
	}

	writeJSON(w, http.StatusOK, out)
}

// ChangesResponse is the site-wide "what's new" feed.
type ChangesResponse struct {
	Since time.Time `json:"since"`
	// CamerasAdded counts camera locations first seen in the window. The
	// importer only inserts and updates, so created_at is a reliable record
	// of when a camera first appeared and nothing overwrites it.
	CamerasAdded int `json:"cameras_added"`
	// ByState is where those cameras landed, busiest first.
	ByState []stateChange `json:"by_state"`
	// Decisions are the moderator actions in the same window.
	Decisions []deploymentEvent `json:"decisions"`
}

type stateChange struct {
	State   string `json:"state"`
	Cameras int    `json:"cameras"`
}

// handleChanges serves GET /stats/changes?days=N.
func (s *Server) handleChanges(w http.ResponseWriter, r *http.Request) {
	days := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= 365 {
		days = v
	}
	since := time.Now().UTC().AddDate(0, 0, -days)
	out := ChangesResponse{Since: since, ByState: []stateChange{}, Decisions: []deploymentEvent{}}

	// Counted separately rather than summed from the per-state rows below.
	// A camera whose state could not be resolved still arrived, and defining
	// the headline figure as the sum of the breakdown would quietly drop it —
	// the total would disagree with the map for reasons no reader could see.
	if err := s.db.QueryRow(r.Context(),
		`SELECT count(*)::int FROM camera_sightings WHERE created_at >= $1`, since,
	).Scan(&out.CamerasAdded); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load recent changes", err)
		return
	}

	rows, err := s.db.Query(r.Context(), `
		SELECT state, count(*)::int
		FROM camera_sightings
		WHERE created_at >= $1 AND state IS NOT NULL
		GROUP BY state
		ORDER BY count(*) DESC, state
		LIMIT 60
	`, since)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load recent changes", err)
		return
	}
	for rows.Next() {
		var sc stateChange
		if err := rows.Scan(&sc.State, &sc.Cameras); err != nil {
			rows.Close()
			serverError(w, r, http.StatusInternalServerError, "could not load recent changes", err)
			return
		}
		out.ByState = append(out.ByState, sc)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load recent changes", err)
		return
	}

	evRows, err := s.db.Query(r.Context(), `
		SELECT e.kind, e.from_status, e.to_status, e.evidence_type,
		       e.source_links, e.note, e.created_at,
		       d.agency_name, d.state, d.slug
		FROM deployment_events e
		JOIN deployments d ON d.id = e.deployment_id
		WHERE e.created_at >= $1
		ORDER BY e.created_at DESC
		LIMIT 50
	`, since)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load recent changes", err)
		return
	}
	defer evRows.Close()
	for evRows.Next() {
		var e deploymentEvent
		if err := evRows.Scan(&e.Kind, &e.FromStatus, &e.ToStatus, &e.EvidenceType,
			&e.SourceLinks, &e.Note, &e.CreatedAt,
			&e.AgencyName, &e.State, &e.Slug); err != nil {
			serverError(w, r, http.StatusInternalServerError, "could not load recent changes", err)
			return
		}
		out.Decisions = append(out.Decisions, e)
	}
	if err := evRows.Err(); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load recent changes", err)
		return
	}

	// Moves only when the weekly import runs or a moderator acts.
	w.Header().Set("Cache-Control", "public, max-age=900")
	writeJSON(w, http.StatusOK, out)
}
