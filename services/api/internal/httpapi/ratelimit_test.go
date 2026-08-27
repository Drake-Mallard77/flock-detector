package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stubDB stands in for Postgres so the limiter's own logic can be tested
// without a database.
type stubDB struct {
	count int
	err   error
	calls int
}

type stubRow struct {
	count int
	err   error
}

func (r stubRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if p, ok := dest[0].(*int); ok {
		*p = r.count
	}
	return nil
}

func (s *stubDB) QueryRow(_ context.Context, _ string, _ ...any) rowScanner {
	s.calls++
	if s.err != nil {
		return stubRow{err: s.err}
	}
	s.count++
	return stubRow{count: s.count}
}

func TestRateLimiter_AllowsUpToMax(t *testing.T) {
	db := &stubDB{}
	rl := newSubmissionRateLimiter(db, time.Minute, 3)

	for i := 1; i <= 3; i++ {
		ok, err := rl.allow(context.Background(), "192.0.2.1")
		if err != nil || !ok {
			t.Fatalf("request %d should be allowed (ok=%v err=%v)", i, ok, err)
		}
	}
	ok, err := rl.allow(context.Background(), "192.0.2.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("the request past the limit should be refused")
	}
}

// A database failure must not open the gate. These are write endpoints, so
// the request needs the database regardless — allowing it through would
// only defer the failure while dropping the flood control at the moment
// things are already going wrong.
func TestRateLimiter_FailsClosed(t *testing.T) {
	db := &stubDB{err: errors.New("connection refused")}
	rl := newSubmissionRateLimiter(db, time.Minute, 10)

	ok, err := rl.allow(context.Background(), "192.0.2.1")
	if err == nil {
		t.Fatal("expected the database error to surface")
	}
	if ok {
		t.Error("must not allow the request when the limiter cannot check")
	}

	rec := doRequest(t, rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when the limiter is unavailable, got %d", rec.Code)
	}
}

func TestRateLimiter_Middleware429(t *testing.T) {
	db := &stubDB{}
	rl := newSubmissionRateLimiter(db, time.Minute, 1)
	h := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	if rec := doRequest(t, h); rec.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec.Code)
	}
	rec := doRequest(t, h)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("a 429 should tell the caller when to retry")
	}
}

func doRequest(t *testing.T, h http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/deployments", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
