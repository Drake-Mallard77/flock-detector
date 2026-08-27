package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// captureLogs swaps the default logger for one writing JSON into a buffer,
// and restores it afterwards.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

// The whole point of the split: the operator gets the cause, the client
// does not. A database error can carry table names, column names, and
// occasionally fragments of the query — none of which belong in a response
// to an anonymous caller on a public site.
func TestServerError_LogsCauseButDoesNotLeakIt(t *testing.T) {
	buf := captureLogs(t)

	r := chi.NewRouter()
	r.Get("/boom", func(w http.ResponseWriter, r *http.Request) {
		serverError(w, r, http.StatusInternalServerError, "could not load deployments",
			errors.New(`ERROR: relation "secret_internal_table" does not exist`))
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret_internal_table") {
		t.Errorf("the internal error leaked to the client: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "could not load deployments") {
		t.Errorf("client should get the safe message, got: %s", rec.Body.String())
	}

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log entry was not valid JSON: %v (%s)", err, buf.String())
	}
	if !strings.Contains(entry["error"].(string), "secret_internal_table") {
		t.Error("the operator-facing log should carry the real cause")
	}
	// Without the route pattern a spike of errors can't be attributed to an
	// endpoint, which is the first question anyone asks.
	if entry["route"] != "/boom" {
		t.Errorf("expected route /boom, got %v", entry["route"])
	}
}

func TestRecoverer_TurnsPanicInto500(t *testing.T) {
	buf := captureLogs(t)

	h := recoverer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("something exploded")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "something exploded") {
		t.Errorf("panic detail leaked to the client: %s", rec.Body.String())
	}
	if !strings.Contains(buf.String(), "something exploded") {
		t.Error("the panic should be logged")
	}
	if !strings.Contains(buf.String(), "stack_trace") {
		t.Error("a panic without a stack trace is nearly useless to debug")
	}
}

// ErrAbortHandler is how a handler deliberately gives up on a response.
// Reporting it as a crash would invent alerts out of normal behaviour.
func TestRecoverer_PassesThroughAbortHandler(t *testing.T) {
	h := recoverer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		if rvr := recover(); rvr != http.ErrAbortHandler {
			t.Errorf("expected ErrAbortHandler to propagate, got %v", rvr)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

// Uptime checks hit /health every minute from several regions. At info
// level they'd drown out real traffic in the log, so they're demoted.
func TestRequestLogger_DemotesHealthChecks(t *testing.T) {
	var buf *bytes.Buffer
	run := func(path string) map[string]any {
		buf = captureLogs(t)
		h := requestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
		var entry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("log entry for %s was not valid JSON: %v", path, err)
		}
		return entry
	}

	if lvl := run("/health")["level"]; lvl != "DEBUG" {
		t.Errorf("/health should log at DEBUG, got %v", lvl)
	}
	if lvl := run("/deployments")["level"]; lvl != "INFO" {
		t.Errorf("/deployments should log at INFO, got %v", lvl)
	}
}

// Cloud Logging reads specific field names. Getting these wrong doesn't
// fail loudly — entries just show up blank or unseverified in the console,
// which is exactly the kind of bug that goes unnoticed until an incident.
func TestSetupLogging_ProductionUsesCloudLoggingFieldNames(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	for _, tc := range []struct {
		level slog.Level
		want  string
	}{
		{slog.LevelWarn, "WARNING"},
		{slog.LevelError, "ERROR"},
	} {
		var b bytes.Buffer
		slog.New(productionHandler(&b)).Log(t.Context(), tc.level, "hello")

		var entry map[string]any
		if err := json.Unmarshal(b.Bytes(), &entry); err != nil {
			t.Fatalf("not JSON: %v", err)
		}
		if entry["severity"] != tc.want {
			t.Errorf("expected severity %q, got %v", tc.want, entry["severity"])
		}
		if entry["message"] != "hello" {
			t.Errorf("expected the text under \"message\", got %v", entry)
		}
		if _, stillDefault := entry["msg"]; stillDefault {
			t.Error(`"msg" should have been renamed to "message"`)
		}
	}
}
