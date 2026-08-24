package httpapi

import (
	"net/http"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestSubmissionRateLimiter_AllowsBurstThenBlocks(t *testing.T) {
	rl := newSubmissionRateLimiter(rate.Limit(1.0/60.0), 3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.allow("192.0.2.1") {
			t.Fatalf("request %d within burst should be allowed", i)
		}
	}
	if rl.allow("192.0.2.1") {
		t.Fatal("4th request beyond burst should be blocked")
	}

	// A different IP has its own independent bucket.
	if !rl.allow("192.0.2.2") {
		t.Fatal("a different IP should not be affected by another IP's rate limit")
	}
}

func TestSubmissionRateLimiter_MiddlewareReturns429(t *testing.T) {
	s := newTestServer(t)
	s.submissionLimiter = newSubmissionRateLimiter(rate.Limit(1.0/60.0), 1, time.Minute)
	h := s.Router()

	// First submission consumes the single burst slot.
	rec := doJSON(t, h, http.MethodPost, "/deployments", map[string]any{
		"agency_name": "Agency A", "city": "Springfield", "state": "IL",
		"evidence_type": "council_report",
	}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("first submission: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Second submission from the same IP should be rate limited.
	rec = doJSON(t, h, http.MethodPost, "/deployments", map[string]any{
		"agency_name": "Agency B", "city": "Springfield", "state": "IL",
		"evidence_type": "council_report",
	}, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second submission: expected 429, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("expected a Retry-After header on a 429 response")
	}

	// GET requests are not rate limited by the submission limiter.
	rec = doJSON(t, h, http.MethodGet, "/deployments", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET should be unaffected by the submission rate limit, got %d", rec.Code)
	}
}
