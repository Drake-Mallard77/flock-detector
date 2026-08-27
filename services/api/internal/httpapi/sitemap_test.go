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
		published[st] = insert(st)
	}
	hidden := map[string]string{}
	for _, st := range []string{"under_review", "disputed", "removed"} {
		hidden[st] = insert(st)
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

	for _, p := range []string{"/", "/deployments", "/methodology", "/submit"} {
		if !locs["https://example.test"+p] {
			t.Errorf("static route %s missing from the sitemap", p)
		}
	}
	// The moderator queue is auth-gated; recommending it to crawlers is
	// pointless at best.
	if locs["https://example.test/review"] {
		t.Error("/review must not be in the sitemap")
	}

	for st, id := range published {
		if !locs["https://example.test/deployments/"+id] {
			t.Errorf("%s record should be listed", st)
		}
	}
	// The point of the status filter: unvetted, contested, and retracted
	// records naming real agencies are not promoted to search engines.
	for st, id := range hidden {
		if locs["https://example.test/deployments/"+id] {
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
