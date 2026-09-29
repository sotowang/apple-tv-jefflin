package api

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"online-media/media-source-server/internal/config"
	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/repository"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeProvider struct{}

func (fakeProvider) Name() string { return "archive" }
func (fakeProvider) Search(ctx context.Context, q media.SearchQuery) ([]media.Media, error) {
	return []media.Media{{ID: "archive:test-api-film", Type: "movie", Title: "Test Film", Provider: "archive", ExternalID: "test-api-film"}}, nil
}
func (fakeProvider) GetMedia(ctx context.Context, id string) (*media.Media, error) {
	return &media.Media{Type: "movie", Title: "Test Film", Provider: "archive", ExternalID: id}, nil
}
func (fakeProvider) GetSources(ctx context.Context, m media.Media) ([]media.Source, error) {
	return []media.Source{{Provider: "archive", ExternalID: m.ExternalID, FileName: "film.mp4", Container: "mp4", DirectPlay: true}}, nil
}
func (fakeProvider) Resolve(ctx context.Context, s media.Source) (*media.ResolvedStream, error) {
	return &media.ResolvedStream{URL: "https://archive.org/download/test-api-film/film.mp4"}, nil
}
func TestAPIIntegration(t *testing.T) {
	db := os.Getenv("TEST_DATABASE_URL")
	if db == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration test")
	}
	ctx := context.Background()
	pool, e := pgxpool.New(ctx, db)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	sql, e := os.ReadFile("../../db/migrations/001_initial.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, string(sql)); e != nil {
		t.Fatal(e)
	}
	a := New(config.Config{APIKey: strings.Repeat("k", 32), SigningSecret: strings.Repeat("s", 32), PublicBaseURL: "https://media.example.com", SearchTTL: time.Minute, MediaTTL: time.Minute}, repository.New(pool), fakeProvider{})
	router := a.Router()
	call := func(method, path, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		if key != "" {
			r.Header.Set("X-API-Key", key)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if w := call("GET", "/api/v1/providers", ""); w.Code != 401 || !strings.Contains(w.Body.String(), "INVALID_API_KEY") {
		t.Fatalf("API key: %d %s", w.Code, w.Body.String())
	}
	key := a.Config.APIKey
	if w := call("GET", "/api/v1/search?q=test", key); w.Code != 200 {
		t.Fatalf("search: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "/api/v1/media/archive:test-api-film", key); w.Code != 200 {
		t.Fatalf("detail: %d %s", w.Code, w.Body.String())
	}
	w := call("GET", "/api/v1/media/archive:test-api-film/sources", key)
	if w.Code != 200 {
		t.Fatalf("sources: %d %s", w.Code, w.Body.String())
	}
	var sr struct {
		Sources []media.Source `json:"sources"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &sr); e != nil || len(sr.Sources) != 1 {
		t.Fatalf("sources json: %v %s", e, w.Body.String())
	}
	w = call("POST", "/api/v1/sources/"+sr.Sources[0].ID+"/play-url", key)
	if w.Code != 200 {
		t.Fatalf("play-url: %d %s", w.Code, w.Body.String())
	}
	var signed struct {
		URL string `json:"url"`
	}
	if e = json.Unmarshal(w.Body.Bytes(), &signed); e != nil {
		t.Fatal(e)
	}
	path := strings.TrimPrefix(signed.URL, "https://media.example.com")
	w = call("GET", path, "")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://archive.org/download/test-api-film/film.mp4" {
		t.Fatalf("play: %d %s %s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
	w = call("GET", path+"x", "")
	if w.Code != 401 {
		t.Fatalf("tamper accepted: %d", w.Code)
	}
}
