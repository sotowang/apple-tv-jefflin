package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"online-media/media-source-server/internal/media"
)

func mockClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := New("test-token", "zh-CN", "CN", time.Second)
	c.BaseURL = srv.URL
	return c
}
func TestChineseSearchMappingAndRanking(t *testing.T) {
	calls := 0
	c := mockClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/search/multi" || r.URL.Query().Get("query") != "凡人修仙传" || r.URL.Query().Get("language") != "zh-CN" || r.URL.Query().Get("region") != "CN" || r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("User-Agent") == "" {
			t.Errorf("bad TMDB request: %s", r.URL.String())
		}
		fmt.Fprint(w, `{"total_pages":1,"results":[{"id":9,"media_type":"tv","name":"凡人修仙传：前传","original_name":"A Record of a Mortal","first_air_date":"2020-01-01","popularity":1000},{"id":123,"media_type":"tv","name":"凡人修仙传","original_name":"A Record of a Mortal's Journey to Immortality","first_air_date":"2025-07-27","poster_path":"/poster.jpg","backdrop_path":"/backdrop.jpg","popularity":1},{"id":2,"media_type":"person","name":"凡人修仙传"}]}`)
	})
	items, err := c.Search(context.Background(), media.SearchQuery{Query: "凡人修仙传", Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(items) != 2 {
		t.Fatalf("calls=%d items=%+v", calls, items)
	}
	got := items[0]
	if got.ID != "tmdb:tv:123" || got.ExternalID != "tv:123" || got.Type != "tv" || got.Title != "凡人修仙传" || got.Year != 2025 || got.PosterURL != "https://image.tmdb.org/t/p/w500/poster.jpg" || got.BackdropURL != "https://image.tmdb.org/t/p/w1280/backdrop.jpg" {
		t.Fatalf("mapping/ranking: %+v", got)
	}
}
func TestMovieSearchAndDetail(t *testing.T) {
	c := mockClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/multi":
			fmt.Fprint(w, `{"total_pages":1,"results":[{"id":42,"media_type":"movie","title":"流浪地球","original_title":"The Wandering Earth","release_date":"2019-02-05","poster_path":"/movie.jpg"}]}`)
		case "/movie/42":
			fmt.Fprint(w, `{"id":42,"title":"流浪地球","original_title":"The Wandering Earth","release_date":"2019-02-05","original_language":"zh","overview":"科幻电影"}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	items, err := c.Search(context.Background(), media.SearchQuery{Query: "流浪地球", Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "tmdb:movie:42" || items[0].Type != "movie" || items[0].Year != 2019 {
		t.Fatalf("movie search: %+v", items)
	}
	detail, err := c.GetMedia(context.Background(), "movie:42")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Title != "流浪地球" || detail.OriginalLanguage != "zh" || detail.ReleaseDate != "2019-02-05" {
		t.Fatalf("movie detail: %+v", detail)
	}
}
func TestTVDetailSeasonsEpisodes(t *testing.T) {
	c := mockClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tv/123":
			fmt.Fprint(w, `{"id":123,"name":"庆余年","first_air_date":"2019-11-26","number_of_seasons":2,"number_of_episodes":82,"seasons":[{"id":11,"season_number":1,"name":"第 1 季","episode_count":46,"poster_path":"/season.jpg","air_date":"2019-11-26"}]}`)
		case "/tv/123/season/1":
			fmt.Fprint(w, `{"id":11,"season_number":1,"episodes":[{"id":111,"episode_number":1,"name":"第 1 集","air_date":"2019-11-26","runtime":45,"still_path":"/still.jpg"}]}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	m, err := c.GetMedia(context.Background(), "tv:123")
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "tmdb:tv:123" || m.SeasonCount != 2 || m.EpisodeCount != 82 {
		t.Fatalf("TV detail: %+v", m)
	}
	seasons, err := c.GetSeasons(context.Background(), "tv:123")
	if err != nil {
		t.Fatal(err)
	}
	if len(seasons) != 1 || seasons[0].SeasonNumber != 1 || seasons[0].EpisodeCount != 46 || seasons[0].PosterURL != "https://image.tmdb.org/t/p/w500/season.jpg" {
		t.Fatalf("seasons: %+v", seasons)
	}
	episodes, err := c.GetEpisodes(context.Background(), "tv:123", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || episodes[0].EpisodeNumber != 1 || episodes[0].RuntimeMinutes != 45 || episodes[0].StillURL != "https://image.tmdb.org/t/p/w780/still.jpg" {
		t.Fatalf("episodes: %+v", episodes)
	}
}
func TestHTTPRetriesAndErrors(t *testing.T) {
	count := 0
	c := mockClient(t, func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		fmt.Fprint(w, `{"id":1,"title":"OK"}`)
	})
	if _, err := c.GetMedia(context.Background(), "movie:1"); err != nil || count != 2 {
		t.Fatalf("retry: count=%d error=%v", count, err)
	}
	c = mockClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "not-json") })
	if _, err := c.GetMedia(context.Background(), "movie:1"); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("decode error: %v", err)
	}
	if _, err := c.GetMedia(context.Background(), "tv:not-an-id"); err == nil {
		t.Fatal("invalid id accepted")
	}
}
