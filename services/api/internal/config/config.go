package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	// Comma-separated, because the site is reachable at more than one
	// origin: the custom domain, its www form, and the Cloud Run URL the
	// custom domain proxies to. A single origin here silently breaks every
	// data request from the others.
	AllowedOrigin string
	Env           string
	// GoogleClientID is the OAuth client the browser signs in against. It
	// doubles as the expected `aud` when validating ID tokens, so an empty
	// value must disable Google sign-in entirely rather than skip the check.
	GoogleClientID string
}

func Load() Config {
	return Config{
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    getenv("DATABASE_URL", "postgres://flockwatch:flockwatch@localhost:5432/flockwatch?sslmode=disable"),
		JWTSecret:      getenv("JWT_SECRET", "dev-secret-change-me"),
		AllowedOrigin:  getenv("ALLOWED_ORIGIN", "http://localhost:5173"),
		Env:            getenv("ENV", "development"),
		GoogleClientID: getenv("GOOGLE_CLIENT_ID", ""),
	}
}

// AllowedOrigins splits ALLOWED_ORIGIN into the list CORS actually needs.
// Blank entries are dropped so a trailing comma or stray whitespace in the
// environment variable can't turn into an empty origin that matches
// nothing (or, worse, is treated as a wildcard by a future change).
func (c Config) AllowedOrigins() []string {
	var out []string
	for _, o := range strings.Split(c.AllowedOrigin, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// DevAuthEnabled reports whether the /auth/dev-login stub should be mounted.
// It is a placeholder until real email magic-link/OAuth ships, and is only
// ever enabled outside production.
func (c Config) DevAuthEnabled() bool {
	return c.Env != "production"
}

const defaultJWTSecret = "dev-secret-change-me"

// RequireSecureSecrets reports an error if the server is configured to run
// in production with a default/placeholder secret. JWT_SECRET signs the
// tokens that authorize moderator actions (approving/rejecting/removing
// public records) — deploying with the well-known default would let anyone
// forge a moderator token.
func (c Config) RequireSecureSecrets() error {
	if c.Env == "production" && c.JWTSecret == defaultJWTSecret {
		return fmt.Errorf("JWT_SECRET must be overridden when ENV=production (refusing to start with the default secret)")
	}
	return nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
