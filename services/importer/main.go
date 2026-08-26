// Command importer bootstraps/refreshes the camera_sightings pin layer from
// OpenStreetMap via the Overpass API.
//
// OSM data is ODbL-licensed. Any redistribution of the resulting database
// must credit "© OpenStreetMap contributors" and keep the derived database
// open under the same terms — this must be surfaced on the app's Methodology
// page before launch. See docs/ARCHITECTURE.md.
//
// Usage:
//
//	importer                      # all US states
//	importer -states CA,TX,NY     # specific states
//	importer -dry-run             # fetch and report counts, write nothing
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"flockwatch/importer/internal/importer"
	"flockwatch/importer/internal/overpass"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	var (
		statesFlag = flag.String("states", "", "comma-separated state codes (default: all US states)")
		dryRun     = flag.Bool("dry-run", false, "fetch and report counts without writing to the database")
		// 15s, not a token 1-2s: the public Overpass instance returned 429
		// on back-to-back large-state queries at 5s spacing during testing.
		// The client also retries with backoff on top of this.
		delay = flag.Duration("delay", 15*time.Second, "pause between per-state Overpass queries")
	)
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" && !*dryRun {
		log.Fatal("DATABASE_URL is required (or pass -dry-run)")
	}

	states := importer.USStates
	if *statesFlag != "" {
		states = nil
		for _, s := range strings.Split(*statesFlag, ",") {
			if s = strings.TrimSpace(strings.ToUpper(s)); s != "" {
				states = append(states, s)
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var pool *pgxpool.Pool
	if !*dryRun {
		var err error
		pool, err = pgxpool.New(ctx, databaseURL)
		if err != nil {
			log.Fatalf("connect to database: %v", err)
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			log.Fatalf("ping database: %v", err)
		}
	}

	client := overpass.New()
	var total importer.Stats

	for i, state := range states {
		if ctx.Err() != nil {
			log.Println("interrupted, stopping")
			break
		}

		// Be a good citizen on the shared public Overpass instance.
		if i > 0 {
			select {
			case <-time.After(*delay):
			case <-ctx.Done():
				log.Println("interrupted, stopping")
				return
			}
		}

		nodes, err := client.FlockALPRNodesInState(ctx, state)
		if err != nil {
			// One state failing (timeout, rate limit) shouldn't abort the
			// whole run — log it and keep going; the next run picks it up.
			log.Printf("%s: fetch failed: %v", state, err)
			continue
		}

		if *dryRun {
			log.Printf("%s: %d nodes (dry run, nothing written)", state, len(nodes))
			total.Fetched += len(nodes)
			continue
		}

		stats, err := importer.UpsertNodes(ctx, pool, state, nodes)
		if err != nil {
			log.Printf("%s: upsert failed: %v", state, err)
			continue
		}

		log.Printf("%s: fetched=%d inserted=%d updated=%d skipped=%d",
			state, stats.Fetched, stats.Inserted, stats.Updated, stats.Skipped)

		total.Fetched += stats.Fetched
		total.Inserted += stats.Inserted
		total.Updated += stats.Updated
		total.Skipped += stats.Skipped
	}

	log.Printf("done: fetched=%d inserted=%d updated=%d skipped=%d",
		total.Fetched, total.Inserted, total.Updated, total.Skipped)
}
