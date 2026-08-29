package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func readRecord(t *testing.T, id string) (status, evidence string, links []string) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(),
		`SELECT status, evidence_type, source_links FROM deployments WHERE id = $1`, id,
	).Scan(&status, &evidence, &links); err != nil {
		t.Fatalf("read record: %v", err)
	}
	return
}

func TestVerifyDeployment(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")

	id := seedRecord(t, "Detroit Police Department", "Detroit", "MI", "osm_documented")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":           "confirmed",
		"evidence_type":    "council_report",
		"source_links":     []string{"https://example.test/council-minutes.pdf"},
		"documented_units": 450,
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	status, evidence, links := readRecord(t, id)
	if status != "confirmed" {
		t.Errorf("expected confirmed, got %q", status)
	}
	if evidence != "council_report" {
		t.Errorf("expected council_report, got %q", evidence)
	}
	if len(links) != 1 {
		t.Errorf("expected the source link to be stored, got %v", links)
	}

	// The reviewer has to be recorded, or a verification is unattributable.
	var reviewedBy *string
	testPool.QueryRow(context.Background(),
		`SELECT reviewed_by FROM deployments WHERE id = $1`, id).Scan(&reviewedBy)
	if reviewedBy == nil {
		t.Error("verification should record who did it")
	}
}

// The rule the endpoint exists to enforce: "confirmed" has to rest on a
// public record, not on the OpenStreetMap import the record came from.
func TestVerify_RefusesCircularAndUnsourcedEvidence(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")
	id := seedRecord(t, "Somewhere PD", "Somewhere", "TX", "osm_documented")

	// Confirming on the strength of the import it was derived from.
	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":        "confirmed",
		"evidence_type": "osm_import",
		"source_links":  []string{"https://www.openstreetmap.org/copyright"},
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("an OSM import must not verify an OSM-derived record, got %d", rec.Code)
	}

	// Right evidence type, no document behind it.
	rec = doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":        "confirmed",
		"evidence_type": "contract",
		"source_links":  []string{},
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("confirming without a source link should be refused, got %d", rec.Code)
	}

	// Whitespace is not a source link.
	rec = doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":        "confirmed",
		"evidence_type": "contract",
		"source_links":  []string{"   "},
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a blank source link should be refused, got %d", rec.Code)
	}

	// Nor is something that isn't a URL.
	rec = doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":        "confirmed",
		"evidence_type": "contract",
		"source_links":  []string{"the city clerk told me"},
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("a non-URL source should be refused, got %d", rec.Code)
	}

	// After all that, the record must be untouched.
	status, evidence, _ := readRecord(t, id)
	if status != "osm_documented" || evidence != "osm_import" {
		t.Errorf("a refused verification must not modify the record, got %s/%s", status, evidence)
	}
}

// Disputing or removing a record is a different act: it needs no positive
// evidence, because it's a retraction rather than a claim.
func TestVerify_AllowsRetractionWithoutSources(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")
	id := seedRecord(t, "Wrong Agency", "Nowhere", "MT", "osm_documented")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":        "disputed",
		"evidence_type": "osm_import",
		"source_links":  []string{},
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("disputing should not require sources, got %d: %s", rec.Code, rec.Body.String())
	}
	if status, _, _ := readRecord(t, id); status != "disputed" {
		t.Errorf("expected disputed, got %q", status)
	}
}

func TestVerify_RequiresModerator(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	id := seedRecord(t, "Somewhere PD", "Somewhere", "TX", "osm_documented")

	body := map[string]any{
		"status":        "confirmed",
		"evidence_type": "contract",
		"source_links":  []string{"https://example.test/contract.pdf"},
	}
	if rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", body, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without a token, got %d", rec.Code)
	}
	submitter := loginAs(t, s, h, "reader@example.test", "submitter")
	if rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", body, submitter); rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for a non-moderator, got %d", rec.Code)
	}
}

// Omitted fields keep their existing values, so attaching a contract does
// not silently wipe a camera count the record already had.
func TestVerify_PreservesOmittedFields(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")

	var id string
	if err := testPool.QueryRow(context.Background(), `
		INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links, documented_units)
		VALUES ('Buffalo Police Department', 'Buffalo', 'NY', 'osm_documented',
		        'osm_import', ARRAY['https://example.test/osm'], 104)
		RETURNING id
	`).Scan(&id); err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":        "contract_found",
		"evidence_type": "contract",
		"source_links":  []string{"https://example.test/contract.pdf"},
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var units *int
	testPool.QueryRow(context.Background(),
		`SELECT documented_units FROM deployments WHERE id = $1`, id).Scan(&units)
	if units == nil || *units != 104 {
		t.Errorf("documented_units should survive a verification that omits it, got %v", units)
	}

	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out["evidence_type"] != "contract" {
		t.Errorf("response should echo the stored evidence type, got %v", out["evidence_type"])
	}
}
