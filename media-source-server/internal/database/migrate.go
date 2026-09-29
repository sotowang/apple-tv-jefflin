package database

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

func Migrate(ctx context.Context, p *pgxpool.Pool) error {
	b, e := os.ReadFile("/app/migrations/001_initial.sql")
	if e != nil {
		return e
	}
	_, e = p.Exec(ctx, string(b))
	return e
}
