// Command derive-deployments proposes agency-level deployment records from
// OpenStreetMap operator tags, for a human to review.
//
// Everything it creates lands as status='under_review' with
// evidence_type='osm_import'. Nothing becomes a published record without a
// moderator confirming it in the Review Desk — an OSM tag is a lead, and
// publishing it as a records-backed claim about a named police department
// is exactly what this project must not do.
//
// Usage:
//
//	derive-deployments -dry-run     # report what it would propose
//	derive-deployments              # create the candidates
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"flockwatch/importer/internal/derive"
	"flockwatch/importer/internal/importer"
	"flockwatch/importer/internal/overpass"
)

func main() {
	var (
		dryRun     = flag.Bool("dry-run", false, "report candidates without writing them")
		reclassify = flag.Bool("reclassify", false, "recompute operator_type on pending candidates, then exit")
		limit      = flag.Int("limit", 0, "stop after N candidates (0 = no limit)")
	)
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if *reclassify {
		n, err := derive.ReclassifyPending(ctx, pool, func(name string) string {
			return string(importer.ClassifyOperator(name))
		})
		if err != nil {
			log.Fatalf("reclassify: %v", err)
		}
		log.Printf("reclassified %d pending candidate(s)", n)
		return
	}

	candidates, err := derive.FindCandidates(ctx, pool)
	if err != nil {
		log.Fatalf("find candidates: %v", err)
	}

	stats := derive.Stats{Groups: len(candidates)}
	log.Printf("found %d operator/state groups with >= %d cameras",
		len(candidates), derive.MinCameras)

	geo := overpass.NewGeocoder()

	for i, c := range candidates {
		if ctx.Err() != nil {
			log.Println("interrupted, stopping")
			break
		}
		if *limit > 0 && i >= *limit {
			break
		}

		if *dryRun {
			log.Printf("would propose: %-45s %s  (%d cameras)", c.Operator, c.State, c.Cameras)
			continue
		}

		// Geocoding is rate-limited to ~1/sec, so only look up a city for
		// candidates that will actually be created.
		city, err := geo.City(ctx, c.Lat, c.Lng)
		if err != nil {
			log.Printf("%s (%s): geocode failed, skipping: %v", c.Operator, c.State, err)
			stats.Skipped++
			continue
		}
		if city == "" {
			// city is NOT NULL and a wrong city is worse than none — a
			// reviewer can supply it, a fabricated one might go unnoticed.
			log.Printf("%s (%s): no city name found, skipping", c.Operator, c.State)
			stats.Skipped++
			continue
		}

		created, err := derive.Create(ctx, pool, c, city)
		if err != nil {
			log.Printf("%s (%s): %v", c.Operator, c.State, err)
			stats.Skipped++
			continue
		}
		if created {
			stats.Created++
			log.Printf("proposed: %-45s %s, %s (%d cameras)", c.Operator, city, c.State, c.Cameras)
		} else {
			stats.Existing++
		}
	}

	// Runs last, so cameras behind records created in this same pass get
	// linked immediately rather than waiting a week for the next run.
	linked, err := derive.LinkCameras(ctx, pool)
	if err != nil {
		log.Printf("linking cameras to deployments: %v", err)
	} else {
		log.Printf("linked %d camera(s) to their deployment", linked)
	}

	log.Printf("done: groups=%d created=%d already-present=%d skipped=%d linked=%d",
		stats.Groups, stats.Created, stats.Existing, stats.Skipped, linked)
}
