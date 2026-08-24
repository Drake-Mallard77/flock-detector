package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// submissionRateLimiter throttles write endpoints per client IP. This exists
// primarily as a data-integrity control, not a performance one: the main
// threat against a crowdsourced public-records site is someone flooding it
// with fake/defamatory submissions to discredit or dilute the dataset, not
// server load. Every submission still requires moderator approval before it
// becomes public, but a flood still burns moderator time and can be used to
// bury real reports, so it's throttled at the door too.
//
// State is an in-memory map, which is correct for the single-VM deployment
// this project targets (see docs/ARCHITECTURE.md). If the API is ever
// horizontally scaled, this needs to move to a shared store (e.g. Redis) or
// each instance only sees a fraction of a given IP's traffic.
type submissionRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*visitorLimiter

	rate  rate.Limit
	burst int
}

type visitorLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newSubmissionRateLimiter allows `burst` immediate requests per IP, then
// refills at `r` events/sec. It starts a background goroutine that evicts
// entries idle for longer than staleAfter, so long-running processes don't
// accumulate unbounded memory from one-off/rotating client IPs.
func newSubmissionRateLimiter(r rate.Limit, burst int, staleAfter time.Duration) *submissionRateLimiter {
	rl := &submissionRateLimiter{
		limiters: make(map[string]*visitorLimiter),
		rate:     r,
		burst:    burst,
	}
	go rl.cleanupLoop(staleAfter)
	return rl
}

func (rl *submissionRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.limiters[ip]
	if !ok {
		v = &visitorLimiter{limiter: rate.NewLimiter(rl.rate, rl.burst)}
		rl.limiters[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

func (rl *submissionRateLimiter) cleanupLoop(staleAfter time.Duration) {
	ticker := time.NewTicker(staleAfter)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-staleAfter)
		rl.mu.Lock()
		for ip, v := range rl.limiters {
			if v.lastSeen.Before(cutoff) {
				delete(rl.limiters, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// middleware rejects requests over the limit with 429 + Retry-After. It
// keys on RemoteAddr, which by this point in the chain reflects the real
// client IP via middleware.RealIP (see server.go's middleware order).
func (rl *submissionRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "10")
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
