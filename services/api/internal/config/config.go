package config

import (
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

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
