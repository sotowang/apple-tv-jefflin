package archive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"online-media/media-source-server/internal/media"
	"strings"
	"testing"
	"time"
)

func TestSearchAndMetadata(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/advancedsearch.php":
			query := r.URL.Query().Get("q")
			if !strings.Contains(query, "mediatype:movies") || !strings.Contains(query, "title:") {
				t.Errorf("invalid title search query %q", query)
			}
			w.Write([]byte(`{"response":{"docs":[{"identifier":"film-1","title":"Film One","year":"1968"}]}}`))
		case "/metadata/film-1":
			w.Write([]byte(`{"metadata":{"title":"Film One","year":"1968","licenseurl":"https://creativecommons.org/licenses/by/4.0/","rights":"Attribution","collection":["movies"]},"files":[{"name":"film.mp4","format":"MPEG4"},{"name":"poster.jpg"},{"name":"film.srt"},{"name":"bad/name.mp4"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	p := New(s.URL, time.Second, time.Hour)
	items, e := p.Search(context.Background(), media.SearchQuery{Query: "Film", Page: 1, Limit: 20})
	if e != nil || len(items) != 1 || items[0].ID != "archive:film-1" || items[0].Year != 1968 {
		t.Fatalf("search: %#v %v", items, e)
	}
	m, e := p.GetMedia(context.Background(), "film-1")
	if e != nil || m.LicenseURL == "" || !strings.Contains(m.Rights, "movies") || m.RightsStatus != media.RightsVerified {
		t.Fatalf("metadata: %#v %v", m, e)
	}
	sources, e := p.GetSources(context.Background(), *m)
	if e != nil || len(sources) != 1 || sources[0].FileName != "film.mp4" || !sources[0].DirectPlay {
		t.Fatalf("sources: %#v %v", sources, e)
	}
}
func TestFeaturedQueryAndSourceRanking(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/advancedsearch.php":
			q := r.URL.Query().Get("q")
			if q != "mediatype:movies AND collection:opensource_movies AND format:MPEG4" || r.URL.Query().Get("sort[]") != "downloads desc" || strings.Contains(q, "title:") {
				t.Errorf("unexpected browse query %q", q)
			}
			w.Write([]byte(`{"response":{"docs":[{"identifier":"featured-1","title":"Featured Film"}]}}`))
		case "/metadata/featured-1":
			w.Write([]byte(`{"metadata":{"title":"Featured Film"},"files":[{"name":"low.mp4","width":"640","height":"360","size":"1000000"},{"name":"high.mp4","width":1920,"height":1080,"size":4000000,"source":"original"},{"name":"other.m4v","width":"3840","height":"2160"},{"name":"other.mov"},{"name":"other.webm"},{"name":"other.mkv"},{"name":"hevc.mp4","video_codec":"hevc"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	p := New(s.URL, time.Second, time.Hour)
	items, err := p.Featured(context.Background(), 1, 20)
	if err != nil || len(items) != 1 || items[0].ID != "archive:featured-1" {
		t.Fatalf("featured: %#v %v", items, err)
	}
	sources, err := p.GetSources(context.Background(), items[0])
	if err != nil || len(sources) != 7 {
		t.Fatalf("sources: %#v %v", sources, err)
	}
	want := []string{"high.mp4", "low.mp4", "other.m4v", "other.mov", "hevc.mp4", "other.webm", "other.mkv"}
	for i, name := range want {
		if sources[i].FileName != name {
			t.Errorf("rank %d got %q want %q", i, sources[i].FileName, name)
		}
	}
	if sources[0].VideoCodec != "" || !sources[0].DirectPlay || sources[4].DirectPlay || sources[5].DirectPlay {
		t.Fatal("container/codec heuristic incorrect")
	}
}
func TestResolveAndFiltering(t *testing.T) {
	p := New("https://archive.org", time.Second, time.Hour)
	for _, name := range []string{"a.jpg", "a.xml", "a.torrent", "a.srt", "../a.mp4"} {
		if _, ok := playable(name, ""); ok {
			t.Errorf("accepted %q", name)
		}
	}
	v, e := p.Resolve(context.Background(), media.Source{ExternalID: "film-1", FileName: "my film.mp4"})
	if e != nil || v.URL != "https://archive.org/download/film-1/my%20film.mp4" || v.ProxyRequired {
		t.Fatalf("resolve: %#v %v", v, e)
	}
}
func TestUpstreamStatusAndLimit(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer s.Close()
	_, e := New(s.URL, time.Second, time.Hour).Search(context.Background(), media.SearchQuery{Query: "x", Page: 1, Limit: 1})
	if e == nil {
		t.Fatal("expected upstream error")
	}
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 5<<20))) }))
	defer s2.Close()
	_, e = New(s2.URL, time.Second, time.Hour).Search(context.Background(), media.SearchQuery{Query: "x", Page: 1, Limit: 1})
	if e == nil {
		t.Fatal("expected response limit")
	}
}
