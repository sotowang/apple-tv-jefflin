package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"os"
	"time"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		slog.Error("database", "error", e)
		os.Exit(1)
	}
	defer p.Close()
	b, e := os.ReadFile("/app/migrations/001_initial.sql")
	if e != nil {
		slog.Error("migration file", "error", e)
		os.Exit(1)
	}
	_, e = p.Exec(ctx, string(b))
	if e != nil {
		slog.Error("migration", "error", e)
		os.Exit(1)
	}
	slog.Info("migration complete")
}
