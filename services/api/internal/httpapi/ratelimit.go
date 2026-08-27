package httpapi

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Submission limits, per client IP per window.
//
// Deliberately generous for a person and restrictive for a script: filing
// twenty sourced records in ten minutes by hand is implausible, while a
// flood attempt hits it almost immediately.
const (
	rateLimitWindow = 10 * time.Minute
	rateLimitMax    = 20
)

// submissionRateLimiter throttles write endpoints per client IP.
//
// This is a data-integrity control, not a performance one. The main threat
// against a crowdsourced public-records site is someone flooding it with
// fabricated or defamatory submissions to dilute or discredit the dataset.
// Every submission still needs moderator approval before it goes public,
// but a flood burns moderator attention and can bury real reports, so it's
// throttled at the door as well.
//
// State lives in Postgres so the limit holds across Cloud Run instances.
// The previous in-memory version gave each instance its own counters, so
// the real limit was max_instance_count times the intended one.
type submissionRateLimiter struct {
	db     dbExecutor
	window time.Duration
	max    int
}

// dbExecutor is the slice of pgxpool.Pool this needs, so tests can supply a
// stub without standing up a pool.
type dbExecutor interface {
	QueryRow(ctx context.Context, sql string, args ...any) rowScanner
}

type rowScanner interface {
	Scan(dest ...any) error
}

func newSubmissionRateLimiter(db dbExecutor, window time.Duration, max int) *submissionRateLimiter {
	return &submissionRateLimiter{db: db, window: window, max: max}
}

// allow records a hit and reports whether the caller is still under the
// limit, using a fixed window.
//
// A fixed window can let up to 2x the limit through across a boundary. That
// is a known and accepted tradeoff here: it's a flood control, not a
// billing meter, and a sliding window costs more complexity than the
// precision is worth.
func (rl *submissionRateLimiter) allow(ctx context.Context, ip string) (bool, error) {
	windowStart := time.Now().UTC().Truncate(rl.window)

	var count int
	err := rl.db.QueryRow(ctx, `
		INSERT INTO rate_limits (key, window_start, count)
		VALUES ($1, $2, 1)
		ON CONFLICT (key) DO UPDATE SET
			count = CASE
				WHEN rate_limits.window_start = EXCLUDED.window_start
				THEN rate_limits.count + 1
				ELSE 1
			END,
			window_start = EXCLUDED.window_start
		RETURNING count
	`, ip, windowStart).Scan(&count)
	if err != nil {
		return false, err
	}

	return count <= rl.max, nil
}

func (rl *submissionRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, err := rl.allow(r.Context(), clientIP(r))
		if err != nil {
			// Fail closed. These are write endpoints, so the request needs
			// the database anyway — allowing it through on a database error
			// would only defer the failure while removing the flood control
			// at exactly the moment things are already going wrong.
			writeError(w, http.StatusServiceUnavailable, "submissions are temporarily unavailable")
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", "600")
			writeError(w, http.StatusTooManyRequests, "too many submissions, please slow down")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr had no port (unusual, but possible in tests).
		return r.RemoteAddr
	}
	return host
}

// pgxAdapter bridges *pgxpool.Pool to dbExecutor. pgx's QueryRow returns a
// concrete pgx.Row, so it doesn't satisfy the interface directly; wrapping
// keeps the limiter testable without importing pgx into its tests.
type pgxAdapter struct{ pool *pgxpool.Pool }

func (a pgxAdapter) QueryRow(ctx context.Context, sql string, args ...any) rowScanner {
	return a.pool.QueryRow(ctx, sql, args...)
}
