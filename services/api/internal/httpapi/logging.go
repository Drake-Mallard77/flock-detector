package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Observability, shaped for where this actually runs.
//
// Cloud Run captures stdout and stderr. If a line is JSON with the fields
// below, Cloud Logging parses it into a structured entry that can be
// filtered and alerted on; anything else lands as an opaque text blob that
// is only greppable by eye. The standard library's `log` output — which is
// what this service used everywhere before — is the opaque kind, so the
// "API returning server errors" alert could tell us that 5xx was up while
// offering nothing to explain why.
//
// Deliberately not Sentry or similar: this is a one-service deployment
// already inside GCP, and Cloud Error Reporting groups these entries for
// free, with no extra vendor, no extra secret to rotate, and nothing new
// that can itself go down. Worth revisiting if the service ever runs
// somewhere else.
const (
	// Marks an entry for Cloud Error Reporting, which groups by message and
	// stack rather than leaving each occurrence to be found by hand.
	reportedErrorType = "type.googleapis.com/google.devtools.clouderrorreporting.v1beta1.ReportedErrorEvent"
)

// SetupLogging installs the default structured logger.
//
// Production emits Cloud Logging JSON. Development emits human-readable
// text, because nobody wants to read JSON in a terminal.
func SetupLogging(env string) {
	if env != "production" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
		return
	}

	slog.SetDefault(slog.New(productionHandler(os.Stdout)))
}

// productionHandler is split out so tests can assert on the emitted JSON
// without capturing os.Stdout.
func productionHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.MessageKey:
				// Cloud Logging promotes "message" to the entry summary; the
				// default "msg" is left as an ordinary payload field, which
				// makes every entry display as blank in the console.
				a.Key = "message"
			case slog.LevelKey:
				a.Key = "severity"
				// Cloud Logging's enum spells it WARNING. "WARN" is not
				// recognised and silently degrades to DEFAULT severity, so
				// warnings would not match a severity>=WARNING filter.
				if lvl, ok := a.Value.Any().(slog.Level); ok && lvl == slog.LevelWarn {
					a.Value = slog.StringValue("WARNING")
				}
			}
			return a
		},
	})
}

// requestLogger replaces chi's middleware.Logger, which writes coloured
// human-readable text. Same information, structured, plus the request ID so
// a request log line and any error logged while serving it can be joined.
//
// Health checks are logged at debug level: uptime checks hit /health every
// minute from several regions, and at info level they would be the
// overwhelming majority of log volume, burying real traffic.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		defer func() {
			level := slog.LevelInfo
			if strings.HasPrefix(r.URL.Path, "/health") {
				level = slog.LevelDebug
			}
			slog.LogAttrs(r.Context(), level, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("remote_ip", clientIP(r)),
			)
		}()

		next.ServeHTTP(ww, r)
	})
}

// recoverer replaces chi's middleware.Recoverer so a panic becomes a
// structured, grouped error rather than a stack trace printed to stderr.
//
// http.ErrAbortHandler is re-panicked rather than reported: it is the
// documented way for a handler to abandon a response, so treating it as a
// crash would manufacture alerts out of normal behaviour.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rvr := recover()
			if rvr == nil {
				return
			}
			if rvr == http.ErrAbortHandler {
				panic(rvr)
			}

			slog.LogAttrs(r.Context(), slog.LevelError, "panic serving request",
				slog.Any("panic", rvr),
				slog.String("stack_trace", string(debug.Stack())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("request_id", middleware.GetReqID(r.Context())),
				slog.String("@type", reportedErrorType),
			)

			// The handler may have written a partial response already; there
			// is nothing useful to do about that beyond not compounding it.
			writeError(w, http.StatusInternalServerError, "something went wrong")
		}()

		next.ServeHTTP(w, r)
	})
}

// serverError logs the real cause and returns a safe message to the client.
//
// The split matters: the client gets a message chosen for a stranger on the
// public internet, while the operator gets the database error, the route,
// and the request ID. Previously the error value was simply dropped at
// every one of these call sites, so "query failed" was all anyone —
// including us — ever saw.
func serverError(w http.ResponseWriter, r *http.Request, status int, clientMessage string, err error) {
	slog.LogAttrs(r.Context(), slog.LevelError, clientMessage,
		slog.String("error", err.Error()),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("route", chi.RouteContext(r.Context()).RoutePattern()),
		slog.Int("status", status),
		slog.String("request_id", middleware.GetReqID(r.Context())),
		slog.String("@type", reportedErrorType),
	)
	writeError(w, status, clientMessage)
}

// logEncodeFailure records a response that failed partway through writing.
// Nothing can be done for the client at that point — the status line is
// already sent — but a truncated sitemap that looks like a success from the
// outside is exactly the kind of failure worth being able to find later.
func logEncodeFailure(r *http.Request, err error) {
	slog.LogAttrs(r.Context(), slog.LevelError, "failed while writing the response body",
		slog.String("error", err.Error()),
		slog.String("path", r.URL.Path),
		slog.String("request_id", middleware.GetReqID(r.Context())),
		slog.String("@type", reportedErrorType),
	)
}
