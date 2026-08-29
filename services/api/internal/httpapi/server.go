package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"flockwatch/api/internal/config"
)

type Server struct {
	db                *pgxpool.Pool
	cfg               config.Config
	submissionLimiter *submissionRateLimiter
}

func NewServer(db *pgxpool.Pool, cfg config.Config) *Server {
	return &Server{
		db:                db,
		cfg:               cfg,
		submissionLimiter: newSubmissionRateLimiter(pgxAdapter{db}, rateLimitWindow, rateLimitMax),
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.cfg.AllowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Named /health, not /healthz: Google's front-end infrastructure (Cloud
	// Run's public routing layer) intercepts requests to /healthz before
	// they reach the container — confirmed while deploying to Cloud Run,
	// see docs/ARCHITECTURE.md.
	r.Get("/health", s.handleHealth)
	// Separate from /health on purpose: the service can be perfectly
	// healthy while the data it serves is weeks out of date, and those
	// warrant different alerts and different responses.
	r.Get("/health/data", s.handleDataFreshness)

	// Served from here rather than as a build-time file in the atlas: the
	// record set refreshes weekly while deploys are occasional, so a static
	// sitemap would advertise a stale snapshot. Caddy proxies
	// /sitemap.xml on the site origin to this.
	r.Get("/sitemap.xml", s.handleSitemap)

	r.Route("/deployments", func(r chi.Router) {
		r.Get("/", s.handleListDeployments)
		r.With(s.submissionLimiter.middleware).Post("/", s.handleCreateDeployment)
		r.Get("/{id}", s.handleGetDeployment)
		// The cameras attributed to one record. Keeps the map and the
		// records from behaving as two unrelated datasets.
		r.Get("/{id}/cameras", s.handleDeploymentCameras)
		r.With(s.requireRole("moderator", "admin")).Post("/{id}/review", s.handleReviewDeployment)
		// Bulk path exists for the OSM-derived candidate queue, which runs
		// to hundreds of records; reviewing those one at a time isn't
		// realistic. Same role gate as the single-record path.
		r.With(s.requireRole("moderator", "admin")).Post("/bulk-review", s.handleBulkReviewDeployments)
	})

	r.Route("/cameras", func(r chi.Router) {
		r.Get("/", s.handleListCameras)
		r.With(s.submissionLimiter.middleware).Post("/", s.handleCreateCamera)
		r.Get("/manufacturers", s.handleListManufacturers)
		// Aggregated counts for zoomed-out views. /cameras returns individual
		// points capped at a fixed limit, which at national zoom showed an
		// arbitrary 0.7% of the data as if it were the whole picture.
		r.Get("/clusters", s.handleCameraClusters)
	})

	r.Route("/auth", func(r chi.Router) {
		// Real sign-in. Rate limited because it's the highest-value
		// endpoint on the site: a moderator session can rewrite the public
		// record, so it shouldn't be freely brute-forceable.
		r.With(s.submissionLimiter.middleware).Post("/google", s.handleGoogleLogin)

		// Who am I? Lets the web app restore a session on reload without
		// trusting anything it stored client-side.
		r.With(s.requireAuth).Get("/me", s.handleMe)

		// Dev-only stub that mints a token for any email/role with no
		// verification. Never mounted when ENV=production — it would be a
		// complete authentication bypass.
		if s.cfg.DevAuthEnabled() {
			r.Post("/dev-login", s.handleDevLogin)
		}
	})

	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
