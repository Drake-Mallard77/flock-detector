package config

import (
	"fmt"
	"os"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	AllowedOrigin string
	Env           string
}

func Load() Config {
	return Config{
		Port:          getenv("PORT", "8080"),
		DatabaseURL:   getenv("DATABASE_URL", "postgres://flockwatch:flockwatch@localhost:5432/flockwatch?sslmode=disable"),
		JWTSecret:     getenv("JWT_SECRET", "dev-secret-change-me"),
		AllowedOrigin: getenv("ALLOWED_ORIGIN", "http://localhost:5173"),
		Env:           getenv("ENV", "development"),
	}
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
