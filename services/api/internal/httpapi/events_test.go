package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
)

// A verification has to leave a trail, and the trail has to record the
// transition rather than just the destination.
func TestVerifyRecordsAnEvent(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")
	id := seedRecord(t, "Detroit Police Department", "Detroit", "MI", "osm_documented")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":        "confirmed",
		"evidence_type": "council_report",
		"source_links":  []string{"https://example.test/minutes.pdf"},
	}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify failed: %d %s", rec.Code, rec.Body.String())
	}

	// Public, deliberately: an audit trail only moderators can read isn't
	// accountability.
	rec = doJSON(t, h, http.MethodGet, "/deployments/"+id+"/events", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var events []deploymentEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Kind != "verified" {
		t.Errorf("expected kind verified, got %q", e.Kind)
	}
	if e.FromStatus == nil || *e.FromStatus != "osm_documented" {
		t.Errorf("event should record where it came from, got %v", e.FromStatus)
	}
	if e.ToStatus != "confirmed" {
		t.Errorf("expected to_status confirmed, got %q", e.ToStatus)
	}
	// The evidence is copied into the event, so a later edit to the record
	// can't silently rewrite what a past decision rested on.
	if len(e.SourceLinks) != 1 {
		t.Errorf("event should capture the evidence it rested on, got %v", e.SourceLinks)
	}
}

// A refused verification must leave no trace: the transaction rolls back
// the record and the event together.
func TestRefusedVerifyRecordsNothing(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")
	id := seedRecord(t, "Somewhere PD", "Somewhere", "TX", "osm_documented")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/verify", map[string]any{
		"status":        "confirmed",
		"evidence_type": "osm_import",
		"source_links":  []string{"https://example.test/x"},
	}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected the circular verification to be refused, got %d", rec.Code)
	}

	rec = doJSON(t, h, http.MethodGet, "/deployments/"+id+"/events", nil, "")
	var events []deploymentEvent
	json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 0 {
		t.Errorf("a refused verification should record nothing, got %d events", len(events))
	}
}

func TestMergeRecordsAnEvent(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")

	survivor := seedRecord(t, "Detroit Police Department", "Detroit", "MI", "osm_documented")
	duplicate := seedRecord(t, "Detroit Police Dept.", "Detroit", "MI", "osm_documented")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+survivor+"/merge",
		map[string]string{"duplicate_id": duplicate}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("merge failed: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/deployments/"+duplicate+"/events", nil, "")
	var events []deploymentEvent
	json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 1 || events[0].Kind != "merged" {
		t.Fatalf("expected a merge event on the retired record, got %+v", events)
	}
	// Where it went is the whole reason to keep this entry.
	if events[0].Note == nil || *events[0].Note == "" {
		t.Error("a merge event should name the record it went into")
	}
}

func TestChangesFeed(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	token := loginAs(t, s, h, "mod@example.test", "moderator")

	dep := seedRecord(t, "Detroit Police Department", "Detroit", "MI", "osm_documented")
	attachCameras(t, dep, 5, "changes")
	doJSON(t, h, http.MethodPost, "/deployments/"+dep+"/verify", map[string]any{
		"status":        "confirmed",
		"evidence_type": "contract",
		"source_links":  []string{"https://example.test/contract.pdf"},
	}, token)

	rec := doJSON(t, h, http.MethodGet, "/stats/changes?days=30", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out ChangesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.CamerasAdded != 5 {
		t.Errorf("expected 5 cameras added, got %d", out.CamerasAdded)
	}
	if len(out.Decisions) != 1 {
		t.Fatalf("expected 1 decision in the feed, got %d", len(out.Decisions))
	}
	// The feed has to name its record, or an entry is unactionable.
	d := out.Decisions[0]
	if d.AgencyName == nil || d.Slug == nil || d.State == nil {
		t.Errorf("feed entries should carry enough to link the record, got %+v", d)
	}
}
