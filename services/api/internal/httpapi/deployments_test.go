package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"flockwatch/api/internal/models"
)

func doJSON(t *testing.T, handler http.Handler, method, target string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	req.RemoteAddr = "192.0.2.1:12345"
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func createDeployment(t *testing.T, h http.Handler, agencyName string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/deployments", map[string]any{
		"agency_name":   agencyName,
		"city":          "Springfield",
		"state":         "IL",
		"evidence_type": "council_report",
		"source_links":  []string{"https://example.gov/doc.pdf"},
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create deployment: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return resp["id"]
}

func loginAs(t *testing.T, s *Server, h http.Handler, email, role string) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/auth/dev-login", map[string]string{
		"email": email, "role": role,
	}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("dev-login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	return resp["token"]
}

func TestHealthz(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s.Router(), http.MethodGet, "/health", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestCreateDeployment_Valid(t *testing.T) {
	s := newTestServer(t)
	id := createDeployment(t, s.Router(), "Springfield PD")
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	rec := doJSON(t, s.Router(), http.MethodGet, "/deployments/"+id, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get deployment: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var d models.Deployment
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode deployment: %v", err)
	}
	if d.Status != models.StatusUnderReview {
		t.Errorf("expected new submission to be under_review, got %q", d.Status)
	}
}

func TestCreateDeployment_MissingRequiredFields(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s.Router(), http.MethodPost, "/deployments", map[string]any{
		"agency_name": "Springfield PD",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateDeployment_InvalidEvidenceType(t *testing.T) {
	s := newTestServer(t)
	rec := doJSON(t, s.Router(), http.MethodPost, "/deployments", map[string]any{
		"agency_name": "Springfield PD", "city": "Springfield", "state": "IL",
		"evidence_type": "not_a_real_type",
	}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("invalid evidence_type")) {
		t.Errorf("expected error message to explain the problem, got: %s", rec.Body.String())
	}
}

func TestListDeployments_Pagination(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	for i := 0; i < 5; i++ {
		createDeployment(t, h, fmt.Sprintf("Agency %d", i))
	}

	rec := doJSON(t, h, http.MethodGet, "/deployments?limit=2&offset=0", nil, "")
	var page1 []models.Deployment
	if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
		t.Fatalf("decode page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 results on page1, got %d", len(page1))
	}

	rec = doJSON(t, h, http.MethodGet, "/deployments?limit=2&offset=2", nil, "")
	var page2 []models.Deployment
	if err := json.Unmarshal(rec.Body.Bytes(), &page2); err != nil {
		t.Fatalf("decode page2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("expected 2 results on page2, got %d", len(page2))
	}
	if page1[0].ID == page2[0].ID || page1[1].ID == page2[0].ID {
		t.Errorf("page1 and page2 should not overlap: page1=%v page2[0]=%v", page1, page2[0])
	}

	rec = doJSON(t, h, http.MethodGet, "/deployments?limit=2&offset=4", nil, "")
	var page3 []models.Deployment
	if err := json.Unmarshal(rec.Body.Bytes(), &page3); err != nil {
		t.Fatalf("decode page3: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("expected 1 result on the last partial page, got %d", len(page3))
	}
}

func TestListDeployments_StateFilter(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	createDeployment(t, h, "Illinois Agency")

	rec := doJSON(t, h, http.MethodGet, "/deployments?state=IL", nil, "")
	var matched []models.Deployment
	json.Unmarshal(rec.Body.Bytes(), &matched)
	if len(matched) != 1 {
		t.Fatalf("expected 1 result filtering state=IL, got %d", len(matched))
	}

	rec = doJSON(t, h, http.MethodGet, "/deployments?state=CA", nil, "")
	var unmatched []models.Deployment
	json.Unmarshal(rec.Body.Bytes(), &unmatched)
	if len(unmatched) != 0 {
		t.Fatalf("expected 0 results filtering state=CA, got %d", len(unmatched))
	}
}

func TestReviewDeployment_RequiresAuth(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	id := createDeployment(t, h, "Springfield PD")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/review", map[string]string{"status": "confirmed"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewDeployment_RequiresModeratorRole(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	id := createDeployment(t, h, "Springfield PD")
	submitterToken := loginAs(t, s, h, "sub@example.com", "submitter")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/review",
		map[string]string{"status": "confirmed"}, submitterToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a submitter role, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewDeployment_InvalidStatus(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	id := createDeployment(t, h, "Springfield PD")
	modToken := loginAs(t, s, h, "mod@example.com", "moderator")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/review",
		map[string]string{"status": "bogus"}, modToken)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an invalid status, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewDeployment_Success(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	id := createDeployment(t, h, "Springfield PD")
	modToken := loginAs(t, s, h, "mod@example.com", "moderator")

	rec := doJSON(t, h, http.MethodPost, "/deployments/"+id+"/review",
		map[string]string{"status": "confirmed"}, modToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, h, http.MethodGet, "/deployments/"+id, nil, "")
	var d models.Deployment
	json.Unmarshal(rec.Body.Bytes(), &d)
	if d.Status != models.StatusConfirmed {
		t.Errorf("expected status confirmed, got %q", d.Status)
	}
	if d.ReviewedBy == nil {
		t.Error("expected reviewed_by to be set")
	}
	if d.LastReviewedAt == nil {
		t.Error("expected last_reviewed_at to be set")
	}
}

// Search runs in SQL, not over the already-fetched page: a client-side
// filter can only see one page of results, so matches further in are
// invisible and the search looks broken.
func TestListDeployments_Search(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	for _, d := range []struct{ agency, city, state string }{
		{"Springfield Police Department", "Springfield", "IL"},
		{"Chicago Police Department", "Chicago", "IL"},
		{"Phoenix Police Department", "Phoenix", "AZ"},
	} {
		rec := doJSON(t, h, http.MethodPost, "/deployments", map[string]any{
			"agency_name": d.agency, "city": d.city, "state": d.state,
			"evidence_type": "council_report",
		}, "")
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %s: %d %s", d.agency, rec.Code, rec.Body.String())
		}
	}

	count := func(query string) int {
		t.Helper()
		rec := doJSON(t, h, http.MethodGet, "/deployments"+query, nil, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", query, rec.Code)
		}
		var got []models.Deployment
		json.Unmarshal(rec.Body.Bytes(), &got)
		return len(got)
	}

	if got := count("?q=Chicago"); got != 1 {
		t.Errorf("q=Chicago: expected 1, got %d", got)
	}
	// Case-insensitive — readers won't match our capitalisation.
	if got := count("?q=chicago"); got != 1 {
		t.Errorf("q=chicago (lowercase): expected 1, got %d", got)
	}
	// Partial words must match; requiring whole words makes search feel broken.
	if got := count("?q=Spring"); got != 1 {
		t.Errorf("q=Spring: expected 1, got %d", got)
	}
	// Matches on state too.
	if got := count("?q=IL"); got != 2 {
		t.Errorf("q=IL: expected 2, got %d", got)
	}
	// Matches on agency name across cities.
	if got := count("?q=Police+Department"); got != 3 {
		t.Errorf("q=Police Department: expected 3, got %d", got)
	}
	if got := count("?q=Nowhere"); got != 0 {
		t.Errorf("q=Nowhere: expected 0, got %d", got)
	}
	// Empty search is not a filter.
	if got := count("?q="); got != 3 {
		t.Errorf("empty q: expected 3, got %d", got)
	}
	// Composes with existing filters.
	if got := count("?q=Police&state=AZ"); got != 1 {
		t.Errorf("q+state: expected 1, got %d", got)
	}
}

func TestBulkReview_RequiresModerator(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	id := createDeployment(t, h, "Springfield PD")

	// Anonymous.
	rec := doJSON(t, h, http.MethodPost, "/deployments/bulk-review",
		map[string]any{"ids": []string{id}, "status": "confirmed"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous: expected 401, got %d", rec.Code)
	}

	// Signed in, but not a moderator. This endpoint can rewrite hundreds of
	// public records in one call, so the role gate matters more here.
	subToken := loginAs(t, s, h, "sub@example.com", "submitter")
	rec = doJSON(t, h, http.MethodPost, "/deployments/bulk-review",
		map[string]any{"ids": []string{id}, "status": "confirmed"}, subToken)
	if rec.Code != http.StatusForbidden {
		t.Errorf("submitter: expected 403, got %d", rec.Code)
	}

	// The record must be untouched by the rejected attempts.
	rec = doJSON(t, h, http.MethodGet, "/deployments/"+id, nil, "")
	var d models.Deployment
	json.Unmarshal(rec.Body.Bytes(), &d)
	if d.Status != models.StatusUnderReview {
		t.Errorf("record changed despite refused auth: %q", d.Status)
	}
}

func TestBulkReview_Success(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	ids := []string{
		createDeployment(t, h, "Agency A"),
		createDeployment(t, h, "Agency B"),
		createDeployment(t, h, "Agency C"),
	}
	modToken := loginAs(t, s, h, "mod@example.com", "moderator")

	rec := doJSON(t, h, http.MethodPost, "/deployments/bulk-review",
		map[string]any{"ids": ids[:2], "status": "confirmed"}, modToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Updated int `json:"updated"`
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Updated != 2 {
		t.Errorf("expected 2 updated, got %d", out.Updated)
	}

	// The two named ids changed and recorded a reviewer...
	for _, id := range ids[:2] {
		rec = doJSON(t, h, http.MethodGet, "/deployments/"+id, nil, "")
		var d models.Deployment
		json.Unmarshal(rec.Body.Bytes(), &d)
		if d.Status != models.StatusConfirmed {
			t.Errorf("%s: expected confirmed, got %q", id, d.Status)
		}
		// Attribution matters more in bulk, not less: one click moved
		// several records and it must stay clear who did it.
		if d.ReviewedBy == nil {
			t.Errorf("%s: reviewed_by not recorded", id)
		}
	}

	// ...and the one left out did not.
	rec = doJSON(t, h, http.MethodGet, "/deployments/"+ids[2], nil, "")
	var untouched models.Deployment
	json.Unmarshal(rec.Body.Bytes(), &untouched)
	if untouched.Status != models.StatusUnderReview {
		t.Errorf("unlisted record was modified: %q", untouched.Status)
	}
}

func TestBulkReview_Validation(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()
	modToken := loginAs(t, s, h, "mod@example.com", "moderator")

	tooMany := make([]string, 201)
	for i := range tooMany {
		tooMany[i] = "00000000-0000-0000-0000-000000000000"
	}

	for name, body := range map[string]map[string]any{
		"empty ids":      {"ids": []string{}, "status": "confirmed"},
		"invalid status": {"ids": []string{"x"}, "status": "bogus"},
		"over the cap":   {"ids": tooMany, "status": "confirmed"},
	} {
		rec := doJSON(t, h, http.MethodPost, "/deployments/bulk-review", body, modToken)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d: %s", name, rec.Code, rec.Body.String())
		}
	}
}
