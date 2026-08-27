package httpapi

import (
	"encoding/xml"
	"net/http"
	"time"
)

// The sitemap protocol caps a single file at 50,000 URLs. The atlas holds
// low thousands of agency-level records, so one file is enough; if this is
// ever hit, the fix is a sitemap index, not a bigger file.
const maxSitemapURLs = 50000

// sitemapStatuses are the records worth recommending to a search engine.
//
// This is narrower than what the site itself shows, and deliberately so. A
// sitemap is a positive recommendation to index, not an access control:
//
//   - under_review is unvetted. These are submissions nobody has checked,
//     and they name real agencies. Handing Google a list of unverified
//     accusations is not the same as displaying one behind an "under
//     review" label, which is what the site does.
//   - disputed is contested by definition — surfacing it as a search result
//     strips the context that makes it honest.
//   - removed was retracted. Advertising a retracted record is the one
//     outcome retraction is meant to prevent.
//
// Those pages stay reachable and are not blocked from crawling; they simply
// aren't promoted. Excluding them from the sitemap is the weakest signal
// available, which is the right strength for a judgment call about
// unverified claims.
var sitemapStatuses = []string{"confirmed", "contract_found", "osm_documented"}

// Static routes worth indexing. /review is omitted because it's the
// moderator queue behind an auth gate, and /deployments/:id pages come from
// the database below.
var staticRoutes = []struct {
	path       string
	changefreq string
	priority   string
}{
	{"/", "daily", "1.0"},
	{"/deployments", "daily", "0.9"},
	{"/methodology", "monthly", "0.5"},
	{"/submit", "monthly", "0.5"},
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type urlSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// handleSitemap serves /sitemap.xml.
//
// Generated from the database rather than written at build time: records
// refresh weekly from OSM while deploys are occasional, so a build-time
// file would advertise a snapshot that is usually stale — which defeats the
// point of submitting one.
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	base := s.cfg.SiteURL

	set := urlSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, rt := range staticRoutes {
		set.URLs = append(set.URLs, sitemapURL{
			Loc:        base + rt.path,
			ChangeFreq: rt.changefreq,
			Priority:   rt.priority,
		})
	}

	rows, err := s.db.Query(r.Context(), `
		SELECT id, greatest(coalesce(last_reviewed_at, updated_at), updated_at)
		FROM deployments
		WHERE status = ANY($1)
		ORDER BY updated_at DESC
		LIMIT $2
	`, sitemapStatuses, maxSitemapURLs-len(staticRoutes))
	if err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not build the sitemap", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var lastmod time.Time
		if err := rows.Scan(&id, &lastmod); err != nil {
			serverError(w, r, http.StatusInternalServerError, "could not build the sitemap", err)
			return
		}
		set.URLs = append(set.URLs, sitemapURL{
			Loc: base + "/deployments/" + id,
			// Date only: the spec allows it, and it avoids implying a record
			// changed at a precision we don't actually track.
			LastMod:    lastmod.UTC().Format("2006-01-02"),
			ChangeFreq: "monthly",
			Priority:   "0.7",
		})
	}
	if err := rows.Err(); err != nil {
		serverError(w, r, http.StatusInternalServerError, "could not build the sitemap", err)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	// Crawlers refetch this often and it's a full table scan of published
	// records; an hour is well inside the weekly refresh cadence.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write([]byte(xml.Header)); err != nil {
		return
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	// The response is already committed with a 200, so a failure here can
	// only be logged — there is no status left to change.
	if err := enc.Encode(set); err != nil {
		logEncodeFailure(r, err)
	}
}
