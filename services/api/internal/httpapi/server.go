package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"

	"flockwatch/api/internal/config"
)

// submissionRate/submissionBurst bound public write endpoints: 3 immediate
// requests per IP, refilling at 1 every 10s. See ratelimit.go for why this
// is treated as a data-integrity control, not just abuse prevention.
const (
	submissionRate       = rate.Limit(1.0 / 10.0)
	submissionBurst      = 3
	submissionStaleAfter = 10 * time.Minute
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
		submissionLimiter: newSubmissionRateLimiter(submissionRate, submissionBurst, submissionStaleAfter),
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{s.cfg.AllowedOrigin},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.Get("/healthz", s.handleHealth)

	r.Route("/deployments", func(r chi.Router) {
		r.Get("/", s.handleListDeployments)
		r.With(s.submissionLimiter.middleware).Post("/", s.handleCreateDeployment)
		r.Get("/{id}", s.handleGetDeployment)
		r.With(s.requireRole("moderator", "admin")).Post("/{id}/review", s.handleReviewDeployment)
	})

	r.Route("/cameras", func(r chi.Router) {
		r.Get("/", s.handleListCameras)
		r.With(s.submissionLimiter.middleware).Post("/", s.handleCreateCamera)
	})

	// Dev-only auth stub: issues a JWT for local testing without wiring up
	// real email magic-link/OAuth yet (tracked for the auth+Review Desk
	// phase). Not mounted with any protection — do not enable in prod.
	if s.cfg.DevAuthEnabled() {
		r.Post("/auth/dev-login", s.handleDevLogin)
	}

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
