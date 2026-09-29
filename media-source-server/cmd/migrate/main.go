package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"online-media/media-source-server/internal/database"
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
	if e = database.Migrate(ctx, p); e != nil {
		slog.Error("migration", "error", e)
		os.Exit(1)
	}
	slog.Info("migration complete")
}
