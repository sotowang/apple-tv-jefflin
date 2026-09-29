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
			if !strings.Contains(r.URL.Query().Get("q"), "mediatype:movies") {
				t.Error("missing media filter")
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
	if e != nil || m.LicenseURL == "" || !strings.Contains(m.Rights, "movies") {
		t.Fatalf("metadata: %#v %v", m, e)
	}
	sources, e := p.GetSources(context.Background(), *m)
	if e != nil || len(sources) != 1 || sources[0].FileName != "film.mp4" || !sources[0].DirectPlay {
		t.Fatalf("sources: %#v %v", sources, e)
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
