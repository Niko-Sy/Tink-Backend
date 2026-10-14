package main

import (
	"chatroombackend/internal/schema"
	"context"
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"os"
	"time"
)

func main() {
	target := flag.Int("target", schema.Latest, "target version")
	apply := flag.Bool("apply", false, "explicitly apply migrations; default only prints version")
	flag.Parse()
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if *apply {
		if e = schema.Apply(ctx, d, *target); e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
	}
	v, dirty, e := schema.Status(ctx, d)
	if e != nil {
		fmt.Fprintln(os.Stderr, "schema marker unavailable: initialize empty databases with --apply; existing unversioned databases require manual baseline verification")
		os.Exit(1)
	}
	fmt.Printf("version=%d dirty=%t target=%d applied=%t\n", v, dirty, *target, *apply)
	if dirty {
		os.Exit(1)
	}
}
