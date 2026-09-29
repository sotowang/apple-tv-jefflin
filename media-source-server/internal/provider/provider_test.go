package provider

import (
	"context"
	"online-media/media-source-server/internal/media"
	"testing"
)

func TestMetadataOnlyMediaHasNoSources(t *testing.T) {
	r := NewRegistry()
	m := media.Media{ID: "tmdb:tv:123", Provider: "tmdb", ExternalID: "tv:123", Type: "tv"}
	sources, err := r.ResolveMedia(context.Background(), m)
	if err != nil || sources == nil || len(sources) != 0 {
		t.Fatalf("sources=%+v error=%v", sources, err)
	}
	episodes, err := r.ResolveEpisode(context.Background(), m, media.Episode{EpisodeNumber: 1})
	if err != nil || episodes == nil || len(episodes) != 0 {
		t.Fatalf("episode sources=%+v error=%v", episodes, err)
	}
}
