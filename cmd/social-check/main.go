package main

import (
	"chatroombackend/internal/schema"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	"os"
	"time"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required; app config is never read")
		os.Exit(1)
	}
	d, e := sql.Open("postgres", dsn)
	if e != nil {
		fmt.Fprintln(os.Stderr, "database unavailable")
		os.Exit(1)
	}
	defer d.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	v, dirty, e := schema.Status(ctx, d)
	if e != nil || dirty || v != schema.Latest {
		fmt.Fprintln(os.Stderr, "inspection requires clean current schema")
		os.Exit(1)
	}
	checks, e := schema.Inspect(ctx, d)
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(checks)
	for _, c := range checks {
		if c.Violations != 0 {
			os.Exit(1)
		}
	}
}
