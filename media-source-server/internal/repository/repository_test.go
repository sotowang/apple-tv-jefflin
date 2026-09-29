package repository

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"online-media/media-source-server/internal/media"
	"os"
	"testing"
)

func TestPersistenceIntegration(t *testing.T) {
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration test")
	}
	ctx := context.Background()
	p, e := pgxpool.New(ctx, u)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	schema, e := os.ReadFile("../../db/migrations/001_initial.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.Exec(ctx, string(schema)); e != nil {
		t.Fatal(e)
	}
	r := New(p)
	id := "test-repository-film"
	m := media.Media{Type: "movie", Title: "First", Provider: "archive", ExternalID: id}
	saved, e := r.UpsertMedia(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	m.Title = "Updated"
	saved, e = r.UpsertMedia(ctx, m)
	if e != nil || saved.Title != "Updated" {
		t.Fatalf("upsert: %#v %v", saved, e)
	}
	got, e := r.MediaByExternal(ctx, "archive", id)
	if e != nil || got.Title != "Updated" {
		t.Fatalf("get: %#v %v", got, e)
	}
	source := media.Source{Provider: "archive", ExternalID: id, FileName: "film.mp4", Container: "mp4", DirectPlay: true}
	first, e := r.UpsertSources(ctx, got, []media.Source{source})
	if e != nil {
		t.Fatal(e)
	}
	second, e := r.UpsertSources(ctx, got, []media.Source{source})
	if e != nil || len(second) != 1 || first[0].ID != second[0].ID {
		t.Fatalf("source upsert: %#v %v", second, e)
	}
	items, e := r.Sources(ctx, "archive", id)
	if e != nil || len(items) != 1 {
		t.Fatalf("source persistence: %#v %v", items, e)
	}
	if e = r.AddLibrary(ctx, got); e != nil {
		t.Fatal(e)
	}
	library, e := r.Library(ctx, 1, 50)
	if e != nil || len(library) == 0 {
		t.Fatalf("library: %#v %v", library, e)
	}
	if e = r.RemoveLibrary(ctx, got); e != nil {
		t.Fatal(e)
	}
	uid, e := r.MediaUUID(ctx, "archive", id)
	if e == nil {
		_, _ = p.Exec(ctx, "DELETE FROM media WHERE id=$1", uid)
	}
}
