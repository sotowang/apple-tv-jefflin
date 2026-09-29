package main

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	"online-media/media-source-server/internal/api"
	"online-media/media-source-server/internal/config"
	"online-media/media-source-server/internal/database"
	"online-media/media-source-server/internal/provider/archive"
	"online-media/media-source-server/internal/repository"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, e := config.Load()
	if e != nil {
		slog.Error("configuration", "error", e)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	pool, e := pgxpool.New(ctx, cfg.DatabaseURL)
	if e != nil {
		slog.Error("database", "error", e)
		os.Exit(1)
	}
	defer pool.Close()
	if e = pool.Ping(ctx); e != nil {
		slog.Error("database ping", "error", e)
		os.Exit(1)
	}
	if e = database.Migrate(ctx, pool); e != nil {
		slog.Error("migration", "error", e)
		os.Exit(1)
	}
	a := api.New(cfg, repository.New(pool), archive.New("https://archive.org", cfg.RequestTimeout, cfg.MediaTTL))
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: a.Router(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			slog.Error("http server", "error", e)
			cancel()
		}
	}()
	<-ctx.Done()
	stop, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	_ = srv.Shutdown(stop)
}
