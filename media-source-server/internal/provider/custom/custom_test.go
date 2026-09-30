package custom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"errors"
	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

func TestMovieAndEpisode(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization %q", got)
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Error("missing accept")
		}
		switch r.URL.Path {
		case "/v1/search/movie":
			if r.URL.Query().Get("title") != "流浪地球" || r.URL.Query().Get("year") != "2019" {
				t.Errorf("movie query %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"items":[{"providerItemId":"abc123","title":"流浪地球","year":2019,"url":"https://example.com/video.m3u8","quality":"1080p","container":"hls","language":"zh-CN","directPlay":true}]}`))
		case "/v1/search/episode":
			q := r.URL.Query()
			if q.Get("title") != "凡人修仙传" || q.Get("season") != "1" || q.Get("episode") != "1" {
				t.Errorf("episode query %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"items":[{"providerItemId":"ep-001","title":"凡人修仙传","season":1,"episode":1,"url":"https://example.com/episode1.m3u8","quality":"1080p","container":"hls","directPlay":true}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()
	p, err := New(upstream.URL, "secret", time.Second, true)
	if err != nil {
		t.Fatal(err)
	}
	movie, err := p.ResolveMedia(context.Background(), media.Media{ID: "tmdb:movie:1", Type: "movie", Title: "流浪地球", Year: 2019})
	if err != nil {
		t.Fatal(err)
	}
	if len(movie) != 1 || movie[0].ProviderItemID != "abc123" || !movie[0].DirectPlay || movie[0].URL != "https://example.com/video.m3u8" {
		t.Fatalf("movie mapping %+v", movie)
	}
	episode, err := p.ResolveEpisode(context.Background(), media.Media{ID: "tmdb:tv:1", Type: "tv", Title: "凡人修仙传"}, media.Episode{SeasonNumber: 1, EpisodeNumber: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(episode) != 1 || episode[0].SeasonNumber != 1 || episode[0].EpisodeNumber != 1 {
		t.Fatalf("episode mapping %+v", episode)
	}
}
func TestStatusTimeoutEmptyAndRedirects(t *testing.T) {
	for _, tc := range []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{"unauthorized", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(401) }, provider.ErrUnauthorized},
		{"empty", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"items":[]}`)) }, nil},
		{"timeout", func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(100 * time.Millisecond)
			_, _ = w.Write([]byte(`{"items":[]}`))
		}, provider.ErrUpstreamUnavailable},
		{"localhost redirect", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "http://localhost/private", 302) }, provider.ErrUpstreamUnavailable},
		{"metadata redirect", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", 302)
		}, provider.ErrUpstreamUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			p, err := New(srv.URL, "", 20*time.Millisecond, true)
			if err != nil {
				t.Fatal(err)
			}
			got, err := p.ResolveMedia(context.Background(), media.Media{Type: "movie", Title: "test"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error %v want %v", err, tc.want)
			}
			if tc.want == nil && len(got) != 0 {
				t.Fatal(got)
			}
		})
	}
	if _, err := New("http://127.0.0.1:8080", "", time.Second, false); err == nil {
		t.Fatal("private HTTP accepted")
	}
}
