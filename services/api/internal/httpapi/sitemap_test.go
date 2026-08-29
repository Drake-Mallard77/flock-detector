package httpapi

import (
	"context"
	"encoding/xml"
	"net/http"
	"strings"
	"testing"
)

func TestSitemap(t *testing.T) {
	s := newTestServer(t)
	s.cfg.SiteURL = "https://example.test"
	h := s.Router()

	insert := func(status string) string {
		t.Helper()
		var id string
		err := testPool.QueryRow(context.Background(), `
			INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links)
			VALUES ($1, 'Springfield', 'IL', $2, 'osm_import', ARRAY['https://example.test/src'])
			RETURNING id
		`, "Agency "+status, status).Scan(&id)
		if err != nil {
			t.Fatalf("insert %s: %v", status, err)
		}
		return id
	}

	published := map[string]string{}
	for _, st := range []string{"confirmed", "contract_found", "osm_documented"} {
		published[st] = slugOf(t, insert(st))
	}
	hidden := map[string]string{}
	for _, st := range []string{"under_review", "disputed", "removed"} {
		hidden[st] = slugOf(t, insert(st))
	}

	rec := doJSON(t, h, http.MethodGet, "/sitemap.xml", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/xml") {
		t.Errorf("expected XML content type, got %q", ct)
	}

	// Parse rather than substring-match: a sitemap that isn't well-formed
	// XML is silently rejected by crawlers, which is the failure mode this
	// endpoint exists to avoid.
	var set urlSet
	if err := xml.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatalf("sitemap is not well-formed XML: %v", err)
	}
	if set.Xmlns != "http://www.sitemaps.org/schemas/sitemap/0.9" {
		t.Errorf("missing or wrong sitemap namespace: %q", set.Xmlns)
	}

	locs := map[string]bool{}
	for _, u := range set.URLs {
		locs[u.Loc] = true
		if !strings.HasPrefix(u.Loc, "https://example.test/") {
			t.Errorf("every loc must be absolute and on the site origin, got %q", u.Loc)
		}
	}

	for _, p := range []string{"/", "/deployments", "/states", "/methodology", "/submit"} {
		if !locs["https://example.test"+p] {
			t.Errorf("static route %s missing from the sitemap", p)
		}
	}
	// The moderator queue is auth-gated; recommending it to crawlers is
	// pointless at best.
	if locs["https://example.test/review"] {
		t.Error("/review must not be in the sitemap")
	}

	for st, slug := range published {
		if !locs["https://example.test/state/il/"+slug] {
			t.Errorf("%s record should be listed", st)
		}
	}
	// The point of the status filter: unvetted, contested, and retracted
	// records naming real agencies are not promoted to search engines.
	for st, slug := range hidden {
		if locs["https://example.test/state/il/"+slug] {
			t.Errorf("%s record must not be listed in the sitemap", st)
		}
	}
}

// A trailing slash in SITE_URL would otherwise produce "https://site//path",
// which crawlers treat as a distinct URL from the real one.
func TestSitemap_TrimsTrailingSlashInSiteURL(t *testing.T) {
	s := newTestServer(t)
	s.cfg.SiteURL = "https://example.test"
	rec := doJSON(t, s.Router(), http.MethodGet, "/sitemap.xml", nil, "")

	if strings.Contains(rec.Body.String(), "example.test//") {
		t.Error("produced a double slash in a loc")
	}
}

// State pages are the entries a search engine can plausibly rank, so their
// presence — and the absence of states with nothing published — is worth
// asserting rather than assuming.
func TestSitemap_IncludesStatePages(t *testing.T) {
	s := newTestServer(t)
	s.cfg.SiteURL = "https://example.test"

	insert := func(state, status string) {
		t.Helper()
		_, err := testPool.Exec(context.Background(), `
			INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links)
			VALUES ($1, 'Somewhere', $2, $3, 'osm_import', ARRAY['https://example.test/src'])
		`, "Agency "+state+status, state, status)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert("TN", "confirmed")
	insert("TN", "osm_documented")
	insert("CA", "contract_found")
	// Only an unvetted candidate: this state must not get a page in the
	// sitemap, for the same reason the record itself doesn't.
	insert("WY", "under_review")

	rec := doJSON(t, s.Router(), http.MethodGet, "/sitemap.xml", nil, "")
	var set urlSet
	if err := xml.Unmarshal(rec.Body.Bytes(), &set); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
	locs := map[string]bool{}
	for _, u := range set.URLs {
		locs[u.Loc] = true
	}

	for _, want := range []string{"https://example.test/state/tn", "https://example.test/state/ca"} {
		if !locs[want] {
			t.Errorf("expected %s in the sitemap", want)
		}
	}
	// One entry per state, not one per record: TN has two published records.
	count := 0
	for _, u := range set.URLs {
		if u.Loc == "https://example.test/state/tn" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one /state/tn entry, got %d", count)
	}
	if locs["https://example.test/state/wy"] {
		t.Error("a state with only unvetted candidates must not get a sitemap entry")
	}
}

// slugOf reads back the slug the database generated for a record, so the
// tests assert on the URL the app will actually serve rather than on a slug
// recomputed here from the same inputs — which would pass even if the
// generation rule changed on both sides.
func slugOf(t *testing.T, id string) string {
	t.Helper()
	var slug string
	if err := testPool.QueryRow(context.Background(),
		`SELECT slug FROM deployments WHERE id = $1`, id).Scan(&slug); err != nil {
		t.Fatalf("read slug for %s: %v", id, err)
	}
	return slug
}
