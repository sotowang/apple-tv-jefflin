package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"online-media/media-source-server/internal/catalog"
	"online-media/media-source-server/internal/provider/archive"
)

func main() {
	limit := 20
	if raw := os.Getenv("CATALOG_LIMIT"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil {
			slog.Error("invalid CATALOG_LIMIT", "error", err)
			os.Exit(1)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	provider := archive.New("https://archive.org", 15*time.Second, 30*time.Minute)
	directory := os.Getenv("CATALOG_DIR")
	if directory == "" {
		directory = "/catalog"
	}
	count, err := catalog.Sync(ctx, provider, directory, limit)
	if err != nil {
		slog.Error("catalog sync failed", "error", err, "written", count)
		os.Exit(1)
	}
	slog.Info("catalog sync complete", "movies", count)
}
