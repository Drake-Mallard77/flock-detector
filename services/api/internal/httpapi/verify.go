package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"flockwatch/api/internal/models"
)

// Verification: promoting an OSM-derived record to one backed by public
// records.
//
// Every one of the 1,142 published records currently carries
// evidence_type='osm_import' and status='osm_documented' — mapped by
// OpenStreetMap contributors and explicitly not checked against anything a
// public body filed. The site says so plainly, which is honest, but it also
// means the verification this project exists to do has never happened for a
// single record.
//
// The Review Desk could already set 'confirmed'; it just never showed a
// record you could set it on, because its queue only ever loaded
// status='under_review'. This is the missing half: reach a published
// record, attach the document that backs it, and promote it.

// Evidence types that represent an actual public record. Deliberately
// excludes osm_import and user_photo: confirming a record on the strength
// of the OpenStreetMap import it was derived from is circular, and a
// photograph shows a camera exists, not who bought it or under what
// contract.
var publicRecordEvidence = map[string]bool{
	string(models.EvidenceCouncilReport): true,
	string(models.EvidenceContract):      true,
	string(models.EvidenceInvoice):       true,
	string(models.EvidenceNewsArticle):   true,
	string(models.EvidenceFOIAResponse):  true,
}

type verifyRequest struct {
	Status          string   `json:"status"`
	EvidenceType    string   `json:"evidence_type"`
	SourceLinks     []string `json:"source_links"`
	DocumentedUnits *int     `json:"documented_units"`
	Notes           *string  `json:"notes"`
}

// handleVerifyDeployment serves POST /deployments/{id}/verify (moderator).
//
// Distinct from /review, which is a yes/no on an unvetted candidate. This
// one records *why* a record is being promoted, and refuses to promote
// without that reason — the constraint is the point of the endpoint.
func (s *Server) handleVerifyDeployment(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if !models.IsValidDeploymentStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "invalid status, must be one of: "+joinDeploymentStatuses())
		return
	}
	if !models.IsValidEvidenceType(req.EvidenceType) {
		writeError(w, http.StatusBadRequest, "invalid evidence_type, must be one of: "+joinEvidenceTypes())
		return
	}

	// Cleaned before the checks below so a list of empty strings can't pass
	// for evidence.
	links := make([]string, 0, len(req.SourceLinks))
	for _, l := range req.SourceLinks {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		u, err := url.Parse(l)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			writeError(w, http.StatusBadRequest, "each source link must be an http(s) URL: "+l)
			return
		}
		links = append(links, l)
	}

	// The rule that makes "confirmed" mean something.
	//
	// A record can only claim to be verified against public records if it
	// names a public record and links to it. Without this the status is a
	// button a tired moderator can click at 1am, and the difference between
	// "confirmed" and "osm_documented" — the distinction the whole site
	// rests on — becomes a matter of trust in the reviewer's memory rather
	// than something a reader can check.
	verifying := req.Status == "confirmed" || req.Status == "contract_found"
	if verifying {
		if !publicRecordEvidence[req.EvidenceType] {
			writeError(w, http.StatusBadRequest,
				"verifying a record needs public-records evidence: a council report, contract, "+
					"invoice, news article, or FOIA response. An OpenStreetMap import cannot "+
					"verify a record derived from OpenStreetMap.")
			return
		}
		if len(links) == 0 {
			writeError(w, http.StatusBadRequest,
				"verifying a record needs at least one source link to the document it rests on")
			return
		}
	}

	c, _ := r.Context().Value(claimsContextKey).(*claims)
	var reviewerID *string
	if c != nil {
		reviewerID = &c.Subject
	}

	// coalesce keeps whatever the record already had when a field is
	// omitted: a moderator attaching a contract shouldn't have to restate
	// the camera count to avoid wiping it.
	tag, err := s.db.Exec(r.Context(), `
		UPDATE deployments
		SET status           = $2,
		    evidence_type    = $3,
		    source_links     = $4,
		    documented_units = coalesce($5, documented_units),
		    notes            = coalesce($6, notes),
		    reviewed_by      = $7,
		    last_reviewed_at = now(),
		    updated_at       = now()
		WHERE id = $1
	`, id, req.Status, req.EvidenceType, links, req.DocumentedUnits, req.Notes, reviewerID)
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not save this verification", err)
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "deployment not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":            id.String(),
		"status":        req.Status,
		"evidence_type": req.EvidenceType,
		"source_links":  links,
	})
}
