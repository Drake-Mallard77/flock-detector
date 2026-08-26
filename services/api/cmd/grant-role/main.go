// Command grant-role sets a user's role directly in the database.
//
// Deliberately a CLI and not an API endpoint. Moderator access is what an
// attacker actually wants here — it can approve fabricated records or delete
// real ones — so there is no web path to it at all, not even an admin-only
// one. Granting requires database credentials, which is a meaningfully
// higher bar than compromising a session.
//
// Usage:
//
//	grant-role -email someone@example.com -role moderator
//	grant-role -email someone@example.com -role submitter   # revoke
//	grant-role -list
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"
)

var validRoles = []string{"submitter", "moderator", "admin"}

func main() {
	var (
		email = flag.String("email", "", "email address of the user")
		role  = flag.String("role", "", "one of: submitter, moderator, admin")
		list  = flag.Bool("list", false, "list users with elevated roles")
	)
	flag.Parse()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if *list {
		listElevated(ctx, pool)
		return
	}

	if *email == "" || *role == "" {
		flag.Usage()
		os.Exit(2)
	}
	if !slices.Contains(validRoles, *role) {
		log.Fatalf("invalid role %q; must be one of %v", *role, validRoles)
	}

	// Only updates an existing row: the user must have signed in at least
	// once first. That way a typo'd address fails loudly instead of quietly
	// creating a moderator account nobody controls.
	tag, err := pool.Exec(ctx,
		`UPDATE users SET role = $1 WHERE lower(email) = lower($2)`, *role, *email)
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	if tag.RowsAffected() == 0 {
		log.Fatalf("no user with email %q — they must sign in once before a role can be granted", *email)
	}

	fmt.Printf("%s is now %s\n", *email, *role)
}

func listElevated(ctx context.Context, pool *pgxpool.Pool) {
	rows, err := pool.Query(ctx,
		`SELECT email, role, created_at FROM users WHERE role <> 'submitter' ORDER BY role, email`)
	if err != nil {
		log.Fatalf("query: %v", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var email, role string
		var createdAt any
		if err := rows.Scan(&email, &role, &createdAt); err != nil {
			log.Fatalf("scan: %v", err)
		}
		fmt.Printf("%-40s %-10s %v\n", email, role, createdAt)
		found = true
	}
	if !found {
		fmt.Println("No users have elevated roles.")
	}
}
