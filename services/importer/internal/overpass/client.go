// Package overpass fetches OSM-tagged Flock Safety ALPR camera nodes via the
// public Overpass API. See docs/ARCHITECTURE.md's "Bootstrap data" section
// for the tag schema and ODbL licensing/attribution requirements.
package overpass

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const defaultEndpoint = "https://overpass-api.de/api/interpreter"

// Overpass's usage policy asks clients to identify themselves and avoid
// hammering the shared public instance. Never remove the User-Agent or the
// caller-provided delay between requests (see Client.Delay / cmd main.go).
const userAgent = "flockwatch-importer/1.0 (+https://github.com/drake-mallard77/flock-detector)"

type Client struct {
	Endpoint   string
	HTTPClient *http.Client
}

func New() *Client {
	return &Client{
		Endpoint:   defaultEndpoint,
		HTTPClient: &http.Client{Timeout: 3 * time.Minute},
	}
}

type Node struct {
	ID   int64             `json:"id"`
	Lat  float64           `json:"lat"`
	Lon  float64           `json:"lon"`
	Tags map[string]string `json:"tags"`
}

type response struct {
	Elements []Node `json:"elements"`
}

// ErrRateLimited signals an Overpass 429/504. The public instance enforces
// per-IP slot limits that a fixed inter-request delay doesn't reliably
// avoid (observed: a 429 on the second consecutive large-state query even
// at 5s spacing), so callers should back off and retry rather than treat
// this as a permanent failure for that state.
var ErrRateLimited = errors.New("overpass rate limited")

// ALPRNodesInState queries every node tagged as an ALPR surveillance camera
// within the given two-letter US state code (e.g. "CA"). admin_level 4
// scopes the area match to the state boundary itself, not same-named
// sub-areas.
//
// Deliberately NOT filtered to manufacturer="Flock Safety": measured against
// OSM, that missed ~10% of documented ALPR cameras (DC had 86 Flock out of
// 95 total, the rest Motorola Solutions/Genetec/Leonardo). The manufacturer
// tag is carried through instead so readers can filter to Flock themselves.
//
// Retries with exponential backoff on rate limiting; returns ErrRateLimited
// if it's still being throttled after maxAttempts.
func (c *Client) ALPRNodesInState(ctx context.Context, stateCode string) ([]Node, error) {
	const maxAttempts = 4
	backoff := 30 * time.Second

	for attempt := 1; ; attempt++ {
		nodes, err := c.fetchOnce(ctx, stateCode)
		if err == nil {
			return nodes, nil
		}
		// Retry transient network failures too, not just rate limiting. A
		// full US run makes 51 requests over ~20 minutes, and TLS handshake
		// timeouts were observed mid-run — without this those states are
		// silently skipped and need a manual re-run.
		if !isRetryable(err) || attempt >= maxAttempts {
			return nil, err
		}

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		backoff *= 2
	}
}

// isRetryable reports whether a failed fetch is worth trying again:
// Overpass rate limiting, or a transient network/timeout error. A context
// cancellation is never retried — that's the caller shutting down.
func isRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, ErrRateLimited) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	// TLS handshake timeouts and connection resets surface as plain errors
	// from net/http rather than as net.Error, so match on the text as a
	// fallback. Narrow enough not to swallow genuine query errors.
	msg := err.Error()
	for _, s := range []string{"TLS handshake timeout", "connection reset", "EOF", "no such host"} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

func (c *Client) fetchOnce(ctx context.Context, stateCode string) ([]Node, error) {
	query := fmt.Sprintf(`
		[out:json][timeout:180];
		area["ISO3166-2"="US-%s"]["admin_level"="4"]->.a;
		node(area.a)
			["man_made"="surveillance"]
			["surveillance:type"="ALPR"];
		out body;
	`, stateCode)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint,
		bytes.NewBufferString("data="+query))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("overpass request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// 429 = too many requests; 504 = Overpass's own "gateway timeout / slot
	// unavailable", which it also returns under load rather than only for
	// genuinely slow queries. Both are worth retrying.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusGatewayTimeout {
		return nil, fmt.Errorf("%w (HTTP %d)", ErrRateLimited, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass returned %d: %s", resp.StatusCode, truncate(body, 500))
	}

	var parsed response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return parsed.Elements, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}
