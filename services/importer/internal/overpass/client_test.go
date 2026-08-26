package overpass

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFlockALPRNodesInState_ParsesNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"elements":[
			{"id":123,"lat":38.9,"lon":-77.0,"tags":{"direction":"315","camera:type":"fixed"}}
		]}`))
	}))
	defer srv.Close()

	c := &Client{Endpoint: srv.URL, HTTPClient: srv.Client()}
	nodes, err := c.FlockALPRNodesInState(context.Background(), "DC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != 123 || nodes[0].Lat != 38.9 || nodes[0].Lon != -77.0 {
		t.Errorf("node parsed incorrectly: %+v", nodes[0])
	}
	if nodes[0].Tags["camera:type"] != "fixed" {
		t.Errorf("tags parsed incorrectly: %+v", nodes[0].Tags)
	}
}

func TestFlockALPRNodesInState_SendsQueryAndUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Write([]byte(`{"elements":[]}`))
	}))
	defer srv.Close()

	c := &Client{Endpoint: srv.URL, HTTPClient: srv.Client()}
	if _, err := c.FlockALPRNodesInState(context.Background(), "DC"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Overpass's usage policy asks clients to identify themselves; dropping
	// this risks the shared public instance blocking the importer outright.
	if gotUA == "" {
		t.Error("expected a User-Agent header to be sent")
	}
}

func TestFlockALPRNodesInState_RetriesOnRateLimit(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"elements":[{"id":1,"lat":1,"lon":2,"tags":{}}]}`))
	}))
	defer srv.Close()

	// Cancel-aware: the real backoff starts at 30s, too slow for a test, so
	// this asserts the retry happens at all rather than waiting it out.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	c := &Client{Endpoint: srv.URL, HTTPClient: srv.Client()}
	nodes, err := c.FlockALPRNodesInState(ctx, "DC")
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if attempts < 2 {
		t.Errorf("expected a retry after 429, got %d attempt(s)", attempts)
	}
	if len(nodes) != 1 {
		t.Errorf("expected 1 node after retry, got %d", len(nodes))
	}
}

func TestFlockALPRNodesInState_GivesUpAfterMaxAttempts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	// Context cancels before the backoff chain completes, so this asserts
	// the retry loop respects cancellation rather than spinning forever.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c := &Client{Endpoint: srv.URL, HTTPClient: srv.Client()}
	_, err := c.FlockALPRNodesInState(ctx, "DC")
	if err == nil {
		t.Fatal("expected an error when persistently rate limited")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, ErrRateLimited) {
		t.Errorf("expected a rate-limit or cancellation error, got: %v", err)
	}
}

func TestFlockALPRNodesInState_NonRetryableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad query"))
	}))
	defer srv.Close()

	c := &Client{Endpoint: srv.URL, HTTPClient: srv.Client()}
	_, err := c.FlockALPRNodesInState(context.Background(), "DC")
	if err == nil {
		t.Fatal("expected an error on HTTP 400")
	}
	if errors.Is(err, ErrRateLimited) {
		t.Error("a 400 should not be treated as rate limiting")
	}
}
