package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"online-media/media-source-server/internal/config"
	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
	"online-media/media-source-server/internal/provider/custom"
)

func TestEphemeralPlayURLRedirect(t *testing.T) {
	a := New(config.Config{APIKey: strings.Repeat("k", 32), SigningSecret: strings.Repeat("s", 32), PublicBaseURL: "https://media.example.com", SourceTTL: time.Minute}, nil)
	sources, _ := a.prepareSources([]media.Source{{Provider: "custom", MediaID: "tmdb:tv:1", URL: "https://video.example.com/episode.m3u8?signature=private", DirectPlay: true, Ephemeral: true}})
	if len(sources) != 1 || !strings.HasPrefix(sources[0].ID, "ephemeral:custom:") || strings.Contains(sources[0].ID, "video.example.com") {
		t.Fatalf("source id %q", sources[0].ID)
	}
	router := a.Router()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sources/"+sources[0].ID+"/play-url", nil)
	req.Header.Set("X-API-Key", a.Config.APIKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("play URL %d %s", w.Code, w.Body.String())
	}
	var reply struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	path := strings.TrimPrefix(reply.URL, a.Config.PublicBaseURL)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != 302 || w.Header().Get("Location") != sources[0].URL {
		t.Fatalf("redirect %d %q", w.Code, w.Header().Get("Location"))
	}
	tampered := strings.Replace(path, "ephemeral:custom:", "ephemeral:other:", 1)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tampered, nil))
	if w.Code != 401 {
		t.Fatalf("tampered %d", w.Code)
	}
}

func TestEpisodeSourcesFromCustomAPI(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/search/episode" || r.URL.Query().Get("title") != "凡人修仙传" || r.URL.Query().Get("season") != "1" || r.URL.Query().Get("episode") != "1" {
			t.Errorf("unexpected lookup %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"items":[{"providerItemId":"ep-001","title":"凡人修仙传","season":1,"episode":1,"url":"https://video.example.com/ep1.m3u8","quality":"1080p","container":"hls","directPlay":true}]}`))
	}))
	defer upstream.Close()
	p, err := custom.New(upstream.URL, "", time.Second, true)
	if err != nil {
		t.Fatal(err)
	}
	registry := provider.NewRegistry()
	registry.RegisterSource(p)
	cfg := config.Config{APIKey: strings.Repeat("k", 32), SigningSecret: strings.Repeat("s", 32), SourceTTL: time.Minute}
	a := NewWithRegistry(cfg, nil, registry)
	m := media.Media{ID: "tmdb:tv:123", Provider: "tmdb", ExternalID: "tv:123", Type: "tv", Title: "凡人修仙传"}
	a.MediaCache.Set(m.ID, m, time.Minute)
	a.EpisodesCache.Set(m.ID+":1", []media.Episode{{ID: m.ID + ":season:1:episode:1", SeasonNumber: 1, EpisodeNumber: 1}}, time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/media/tmdb:tv:123/seasons/1/episodes/1/sources", nil)
	req.Header.Set("X-API-Key", cfg.APIKey)
	w := httptest.NewRecorder()
	a.Router().ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Items   []media.Source `json:"items"`
		Sources []media.Source `json:"sources"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != 1 || len(response.Sources) != 1 || response.Items[0].Provider != "custom" || response.Items[0].EpisodeNumber != 1 || !response.Items[0].DirectPlay || response.Items[0].ID == "" {
		t.Fatalf("sources %+v", response)
	}
}
