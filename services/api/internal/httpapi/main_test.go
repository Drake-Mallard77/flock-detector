package httpapi

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/time/rate"

	"flockwatch/api/internal/config"
	"flockwatch/api/internal/db"
)

// testPool is shared across the package's tests: standing up a fresh
// Postgres+PostGIS container per test would make the suite slow. Each test
// calls resetDB(t) first to truncate every table, so tests don't see each
// other's data.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgis/postgis:16-3.4-alpine",
		postgres.WithDatabase("flockwatch_test"),
		postgres.WithUsername("flockwatch"),
		postgres.WithPassword("flockwatch"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "get connection string: %v\n", err)
		os.Exit(1)
	}

	testPool, err = db.Connect(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect to test database: %v\n", err)
		os.Exit(1)
	}
	defer testPool.Close()

	migrationsFS := os.DirFS("../../migrations")
	if err := db.Migrate(ctx, testPool, migrationsFS, "."); err != nil {
		fmt.Fprintf(os.Stderr, "run migrations: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// resetDB truncates every app table so each test starts from a clean slate,
// without paying the cost of a new container per test.
func resetDB(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(),
		`TRUNCATE camera_sightings, deployments, users RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

// newTestServer returns a Server wired to the shared test database, with a
// fresh (generous) rate limiter unless the test overrides it — most tests
// aren't testing rate limiting and shouldn't be flaky because of it.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	resetDB(t)
	return &Server{
		db: testPool,
		cfg: config.Config{
			JWTSecret:     "test-secret",
			AllowedOrigin: "http://localhost:5173",
			Env:           "development",
		},
		submissionLimiter: newSubmissionRateLimiter(rate.Limit(1000), 1000, time.Minute),
	}
}
