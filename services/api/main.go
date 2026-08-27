package main

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flockwatch/api/internal/config"
	"flockwatch/api/internal/db"
	"flockwatch/api/internal/httpapi"
)

//go:embed all:migrations
var migrationsFS embed.FS

func main() {
	cfg := config.Load()
	httpapi.SetupLogging(cfg.Env)
	if err := cfg.RequireSecureSecrets(); err != nil {
		fatal("refusing to start", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("connect to database", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrationsFS, "migrations"); err != nil {
		fatal("run migrations", err)
	}

	server := httpapi.NewServer(pool, cfg)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("flockwatch api listening", "port", cfg.Port, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fatal("http server", err)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}

// fatal reports a startup failure through the same structured pipeline as
// everything else, then exits. log.Fatal wrote plain text, which on Cloud
// Run means a crash-looping container produces log entries with no severity
// — they don't match an error filter and don't raise an alert, so the
// service can be down and quiet at the same time.
func fatal(msg string, err error) {
	slog.Error(msg, "error", err)
	os.Exit(1)
}
