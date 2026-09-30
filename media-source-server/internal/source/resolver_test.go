package source

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

type authError struct{}

func (authError) Error() string       { return "auth failed" }
func (authError) Unwrap() error       { return provider.ErrUnauthorized }
func (authError) UpstreamStatus() int { return 401 }

type fake struct {
	name    string
	sources []media.Source
	err     error
}

func (f fake) Name() string { return f.name }
func (f fake) ResolveMedia(context.Context, media.Media) ([]media.Source, error) {
	return f.sources, f.err
}
func (f fake) ResolveEpisode(context.Context, media.Media, media.Episode) ([]media.Source, error) {
	return f.sources, f.err
}
func (f fake) ResolveStream(context.Context, media.Source) (*media.ResolvedStream, error) {
	return nil, provider.ErrUnsupported
}
func TestFailureIsolationDedupSortAndMatching(t *testing.T) {
	m := media.Media{ID: "tmdb:tv:1", Title: "凡人修仙传", Type: "tv"}
	e := media.Episode{SeasonNumber: 1, EpisodeNumber: 1}
	r := provider.NewRegistry()
	r.RegisterSource(fake{name: "custom", err: provider.ErrUpstreamUnavailable})
	r.RegisterSource(fake{name: "archive", sources: []media.Source{{Provider: "archive", Title: "《凡人 修仙传》", SeasonNumber: 1, EpisodeNumber: 1, URL: "https://example.com/a.m3u8", Quality: "720p", RequiresProxy: true}}})
	got, err := New(r, "custom,archive").ResolveEpisode(context.Background(), m, e)
	if err != nil || len(got) != 1 {
		t.Fatalf("isolation: %v %+v", err, got)
	}
	r.RegisterSource(fake{name: "custom", sources: []media.Source{
		{Provider: "custom", Title: "凡人修仙传", SeasonNumber: 1, EpisodeNumber: 1, URL: "https://example.com/a.m3u8", Quality: "1080p", DirectPlay: true},
		{Provider: "custom", Title: "凡人修仙传", SeasonNumber: 1, EpisodeNumber: 1, URL: "https://example.com/b.m3u8", Quality: "2160p", DirectPlay: true},
		{Provider: "custom", Title: "凡人修仙传", SeasonNumber: 2, EpisodeNumber: 1, URL: "https://example.com/wrong.m3u8", DirectPlay: true},
	}})
	got, err = New(r, "custom,archive").ResolveEpisode(context.Background(), m, e)
	if err != nil || len(got) != 2 || got[0].URL != "https://example.com/b.m3u8" {
		t.Fatalf("dedup/sort: %v %+v", err, got)
	}
}
func TestAllFailedAndEmpty(t *testing.T) {
	r := provider.NewRegistry()
	r.RegisterSource(fake{name: "custom", err: provider.ErrUpstreamUnavailable})
	if _, err := New(r, "").ResolveMedia(context.Background(), media.Media{}); !errors.Is(err, provider.ErrUpstreamUnavailable) {
		t.Fatal(err)
	}
	r.RegisterSource(fake{name: "archive", err: provider.ErrUnsupported})
	if _, err := New(r, "").ResolveMedia(context.Background(), media.Media{}); !errors.Is(err, provider.ErrUpstreamUnavailable) {
		t.Fatal(err)
	}
	r.RegisterSource(fake{name: "custom"})
	got, err := New(r, "").ResolveMedia(context.Background(), media.Media{})
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty: %v %+v", err, got)
	}
}

func TestUnauthorizedStructuredLog(t *testing.T) {
	var output bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(old)
	r := provider.NewRegistry()
	r.RegisterSource(fake{name: "custom", err: authError{}})
	_, _ = New(r, "").ResolveEpisode(context.Background(), media.Media{ID: "tmdb:tv:1"}, media.Episode{SeasonNumber: 1, EpisodeNumber: 1})
	logged := output.String()
	for _, part := range []string{`"provider":"custom"`, `"operation":"episode"`, `"mediaId":"tmdb:tv:1"`, `"season":1`, `"episode":1`, `"status":"auth_error"`, `"upstreamStatus":401`} {
		if !bytes.Contains(output.Bytes(), []byte(part)) {
			t.Fatalf("missing %s in %s", part, logged)
		}
	}
}
