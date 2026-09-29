package api

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"online-media/media-source-server/internal/config"
	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
	"online-media/media-source-server/internal/repository"
	"os"
	"strings"
	"testing"
	"time"
)

type fakeProvider struct{}
type fakeTMDB struct{}

func (fakeTMDB) Name() string { return "tmdb" }
func (fakeTMDB) Search(context.Context, media.SearchQuery) ([]media.Media, error) {
	return []media.Media{{ID: "tmdb:tv:987654", Provider: "tmdb", ExternalID: "tv:987654", Type: "tv", Title: "凡人修仙传"}}, nil
}
func (fakeTMDB) GetMedia(context.Context, string) (*media.Media, error) {
	return &media.Media{ID: "tmdb:tv:987654", Provider: "tmdb", ExternalID: "tv:987654", Type: "tv", Title: "凡人修仙传", Year: 2025, ReleaseDate: "2025-01-01", SeasonCount: 1, EpisodeCount: 1, TMDBID: 987654}, nil
}
func (fakeTMDB) GetSeasons(context.Context, string) ([]media.Season, error) {
	return []media.Season{{SeasonNumber: 1, Name: "第 1 季", EpisodeCount: 1, ExternalID: "123"}}, nil
}
func (fakeTMDB) GetEpisodes(context.Context, string, int) ([]media.Episode, error) {
	return []media.Episode{{EpisodeNumber: 1, Name: "第 1 集", ExternalID: "456"}}, nil
}

func TestDebugRouteAbsentInProduction(t *testing.T) {
	const key = "test-api-key"
	router := New(config.Config{AppEnv: "production", APIKey: key}, nil).Router()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/debug/media/archive:test", nil)
	req.Header.Set("X-API-Key", key)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("production debug route status = %d, want 404", w.Code)
	}
}

func (fakeProvider) Name() string { return "archive" }
func (fakeProvider) Search(ctx context.Context, q media.SearchQuery) ([]media.Media, error) {
	return []media.Media{{ID: "archive:test-api-film", Type: "movie", Title: "Test Film", Provider: "archive", ExternalID: "test-api-film"}}, nil
}
func (fakeProvider) Featured(ctx context.Context, page, limit int) ([]media.Media, error) {
	return []media.Media{{ID: "archive:test-api-film", Type: "movie", Title: "Test Film", Provider: "archive", ExternalID: "test-api-film"}}, nil
}
func (fakeProvider) GetMedia(ctx context.Context, id string) (*media.Media, error) {
	return &media.Media{Type: "movie", Title: "Test Film", Provider: "archive", ExternalID: id, LicenseURL: "https://creativecommons.org/licenses/by/4.0/"}, nil
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
	sql, e = os.ReadFile("../../db/migrations/002_media_v2.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = pool.Exec(ctx, string(sql)); e != nil {
		t.Fatal(e)
	}
	_, _ = pool.Exec(ctx, "DELETE FROM media WHERE external_id=$1", "test-api-film")
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
	if w := call("GET", "/api/v1/featured?limit=20", key); w.Code != 200 || !strings.Contains(w.Body.String(), "test-api-film") {
		t.Fatalf("featured: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "/api/v1/media/archive:test-api-film", key); w.Code != 200 || !strings.Contains(w.Body.String(), `"rightsStatus":"verified"`) {
		t.Fatalf("detail: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "/api/v1/debug/media/archive:test-api-film", key); w.Code != 200 || !strings.Contains(w.Body.String(), `"host":"archive.org"`) || strings.Contains(w.Body.String(), "https://archive.org/download/") {
		t.Fatalf("debug: %d %s", w.Code, w.Body.String())
	}
	prod := New(config.Config{AppEnv: "production", APIKey: key}, repository.New(pool), fakeProvider{}).Router()
	prodRequest := httptest.NewRequest("GET", "/api/v1/debug/media/archive:test-api-film", nil)
	prodRequest.Header.Set("X-API-Key", key)
	prodResponse := httptest.NewRecorder()
	prod.ServeHTTP(prodResponse, prodRequest)
	if prodResponse.Code != 404 {
		t.Fatalf("production debug exposed: %d", prodResponse.Code)
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
	if w := call("HEAD", path, ""); w.Code != http.StatusFound {
		t.Fatalf("HEAD play: %d", w.Code)
	}
	w = call("GET", path+"x", "")
	if w.Code != 401 {
		t.Fatalf("tamper accepted: %d", w.Code)
	}
	if w := call("GET", "/play/"+sr.Sources[0].ID+"?expires=1&token=bad", ""); w.Code != 401 || !strings.Contains(w.Body.String(), "PLAY_TOKEN_EXPIRED") {
		t.Fatalf("expired token: %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", "/play/"+sr.Sources[0].ID+"?expires=bad&token=bad", ""); w.Code != 401 || !strings.Contains(w.Body.String(), "INVALID_PLAY_TOKEN") {
		t.Fatalf("invalid token: %d %s", w.Code, w.Body.String())
	}
	if w := call("POST", "/api/v1/sources/00000000-0000-0000-0000-000000000001/play-url", key); w.Code != 404 || !strings.Contains(w.Body.String(), "SOURCE_NOT_FOUND") {
		t.Fatalf("source not found: %d %s", w.Code, w.Body.String())
	}
	registry := provider.NewRegistry()
	registry.RegisterLegacy(fakeProvider{})
	registry.RegisterMetadata(fakeTMDB{})
	v2 := NewWithRegistry(a.Config, repository.New(pool), registry).Router()
	v2call := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("X-API-Key", key)
		w := httptest.NewRecorder()
		v2.ServeHTTP(w, req)
		return w
	}
	for _, tc := range []struct{ path, want string }{
		{"/api/v1/search?q=凡人修仙传&provider=tmdb", `"id":"tmdb:tv:987654"`},
		{"/api/v1/media/tmdb:tv:987654", `"seasonCount":1`},
		{"/api/v1/media/tmdb:tv:987654/seasons", `"seasonNumber":1`},
		{"/api/v1/media/tmdb:tv:987654/seasons/1/episodes", `"episodeNumber":1`},
		{"/api/v1/media/tmdb:tv:987654/sources", `"items":[]`},
		{"/api/v1/media/tmdb:tv:987654/seasons/1/episodes/1/sources", `"items":[]`},
	} {
		w := v2call(tc.path)
		if w.Code != 200 || !strings.Contains(w.Body.String(), tc.want) {
			t.Fatalf("V2 %s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	_, _ = pool.Exec(ctx, "DELETE FROM media WHERE external_id=$1", "tv:987654")
}
