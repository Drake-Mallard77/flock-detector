package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Cap on points returned for one record's map. The largest agencies hold a
// few hundred linked cameras; this is a bound against a pathological record
// rather than a limit anyone should hit.
const maxRecordCameras = 2000

type deploymentCamera struct {
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Direction *int    `json:"direction,omitempty"`
}

type deploymentCamerasResponse struct {
	// Linked is how many camera locations are attributed to this agency.
	//
	// Deliberately separate from the record's documented_units. They answer
	// different questions and usually disagree: documented_units is what the
	// evidence says the agency operates, while this is how many of those
	// OpenStreetMap contributors have actually mapped and tagged with an
	// operator. Presenting either as the other would overstate what is known.
	Linked  int                `json:"linked"`
	Cameras []deploymentCamera `json:"cameras"`
}

// handleDeploymentCameras serves GET /deployments/{id}/cameras.
func (s *Server) handleDeploymentCameras(w http.ResponseWriter, r *http.Request) {
	// Validated here rather than letting Postgres reject the cast. A bad id
	// would otherwise surface as a query error indistinguishable from a real
	// database failure, and answering 404 for both is how a broken database
	// gets reported as "not found".
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	rows, err := s.db.Query(r.Context(), `
		SELECT ST_Y(location::geometry), ST_X(location::geometry), direction
		FROM camera_sightings
		WHERE deployment_id = $1
		ORDER BY created_at
		LIMIT $2
	`, id, maxRecordCameras)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load cameras for this record", err)
		return
	}
	defer rows.Close()

	out := deploymentCamerasResponse{Cameras: []deploymentCamera{}}
	for rows.Next() {
		var c deploymentCamera
		if err := rows.Scan(&c.Lat, &c.Lng, &c.Direction); err != nil {
			serverError(w, r, http.StatusInternalServerError, "could not load cameras for this record", err)
			return
		}
		out.Cameras = append(out.Cameras, c)
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not load cameras for this record", err)
		return
	}
	out.Linked = len(out.Cameras)

	writeJSON(w, http.StatusOK, out)
}
