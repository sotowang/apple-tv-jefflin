package catalog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"online-media/media-source-server/internal/media"
)

type testProvider struct{}

func (testProvider) GetMedia(_ context.Context, id string) (*media.Media, error) {
	return &media.Media{Title: "Film & Friends", Year: 1968, Overview: "A <classic>", ExternalID: id, RightsStatus: media.RightsVerified}, nil
}
func (testProvider) GetSources(_ context.Context, _ media.Media) ([]media.Source, error) {
	return []media.Source{{FileName: "movie.mp4", DirectPlay: true}}, nil
}
func (testProvider) Resolve(_ context.Context, _ media.Source) (*media.ResolvedStream, error) {
	return &media.ResolvedStream{URL: "https://archive.org/download/his_girl_friday/movie.mp4"}, nil
}

func TestSyncWritesStandardMovie(t *testing.T) {
	dir := t.TempDir()
	count, err := Sync(context.Background(), testProvider{}, dir, 1)
	if err != nil || count != 1 {
		t.Fatalf("sync: count=%d err=%v", count, err)
	}
	strm, err := os.ReadFile(filepath.Join(dir, "his_girl_friday", "movie.strm"))
	if err != nil || string(strm) != "https://archive.org/download/his_girl_friday/movie.mp4\n" {
		t.Fatalf("strm: %q err=%v", strm, err)
	}
	nfo, err := os.ReadFile(filepath.Join(dir, "his_girl_friday", "movie.nfo"))
	if err != nil || !strings.Contains(string(nfo), "Film &amp; Friends") || !strings.Contains(string(nfo), "A &lt;classic&gt;") {
		t.Fatalf("nfo: %q err=%v", nfo, err)
	}
}

func TestArchiveURL(t *testing.T) {
	for _, raw := range []string{"http://archive.org/download/x/a.mp4", "https://archive.org.evil.test/download/x/a.mp4", "https://evil.test/download/x/a.mp4"} {
		if archiveURL(raw) {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
}
