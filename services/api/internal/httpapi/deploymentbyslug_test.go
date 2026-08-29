package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"flockwatch/api/internal/models"
)

func TestGetDeploymentBySlug(t *testing.T) {
	s := newTestServer(t)
	h := s.Router()

	var id, slug string
	err := testPool.QueryRow(context.Background(), `
		INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links)
		VALUES ('Nashville Metropolitan Police Department',
		        'Nashville', 'TN', 'osm_documented', 'osm_import', ARRAY['https://example.test'])
		RETURNING id, slug
	`).Scan(&id, &slug)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if slug != "nashville-nashville-metropolitan-police-department" {
		t.Errorf("unexpected slug %q", slug)
	}

	rec := doJSON(t, h, http.MethodGet, "/deployments/by-slug/tn/"+slug, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got models.Deployment
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != id {
		t.Errorf("resolved the wrong record: %s vs %s", got.ID, id)
	}
	if got.Slug != slug {
		t.Errorf("slug should travel with the record, got %q", got.Slug)
	}

	// The state in the path arrives lowercase from /state/tn/... while the
	// column stores it uppercase.
	if rec := doJSON(t, h, http.MethodGet, "/deployments/by-slug/TN/"+slug, nil, ""); rec.Code != http.StatusOK {
		t.Errorf("uppercase state should also resolve, got %d", rec.Code)
	}

	if rec := doJSON(t, h, http.MethodGet, "/deployments/by-slug/tn/no-such-agency", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown slug should 404, got %d", rec.Code)
	}
	// Right slug, wrong state: must not resolve, or slugs stop being
	// state-scoped and two states' records collide.
	if rec := doJSON(t, h, http.MethodGet, "/deployments/by-slug/ca/"+slug, nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("slug from another state should 404, got %d", rec.Code)
	}
}

// The generator must never return a slug that's already taken, because the
// column is NOT NULL with a unique index — a collision would fail the insert
// outright rather than degrade.
func TestNextDeploymentSlug_Disambiguates(t *testing.T) {
	s := newTestServer(t)
	_ = s

	insert := func(agency string) string {
		t.Helper()
		var slug string
		err := testPool.QueryRow(context.Background(), `
			INSERT INTO deployments (agency_name, city, state, status, evidence_type, source_links)
			VALUES ($1, 'Nevada County', 'CA', 'osm_documented', 'osm_import',
			        ARRAY['https://example.test'])
			RETURNING slug
		`, agency).Scan(&slug)
		if err != nil {
			t.Fatalf("insert %q: %v", agency, err)
		}
		return slug
	}

	// The real case that produced duplicates: a curly and a straight
	// apostrophe. Both must be storable, at distinct URLs.
	first := insert("Nevada County Sheriff\u2019s Office")
	second := insert("Nevada County Sheriff's Office")

	if first != "nevada-county-nevada-county-sheriff-s-office" {
		t.Errorf("unexpected first slug %q", first)
	}
	if second == first {
		t.Fatalf("second record reused the first slug %q", first)
	}
	if second != first+"-2" {
		t.Errorf("expected a -2 suffix, got %q", second)
	}
}
