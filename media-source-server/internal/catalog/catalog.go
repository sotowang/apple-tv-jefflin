package catalog

import (
	"context"
	"encoding/xml"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"online-media/media-source-server/internal/media"
)

type Provider interface {
	GetMedia(context.Context, string) (*media.Media, error)
	GetSources(context.Context, media.Media) ([]media.Source, error)
	Resolve(context.Context, media.Source) (*media.ResolvedStream, error)
}

// Explicitly reviewed Archive entries. The remote license metadata is checked again
// on every sync; collection membership and download rank are not sufficient.
var curatedIDs = []string{
	"his_girl_friday",
	"The_Pied_Piper_of_Hamelin",
	"TheStranger_0",
	"my_favorite_brunette",
	"gullivers_travels1939",
	"ThePhantomoftheOpera",
	"20000LeaguesUndertheSea",
	"lost_world",
	"royal_wedding",
	"meet_john_doe",
	"little_princess",
	"CarnivalofSouls",
	"BattleshipPotemkin",
	"secret_weapon",
	"Detour",
	"Sita_Sings_the_Blues",
	"TheFlyingDeuces",
	"CC_1916_10_02_ThePawnshop",
	"penny_serenade",
	"ScarletStreet",
}

type movieNFO struct {
	XMLName xml.Name `xml:"movie"`
	Title   string   `xml:"title"`
	Year    int      `xml:"year,omitempty"`
	Plot    string   `xml:"plot,omitempty"`
}

// Sync writes small Jellyfin movie-library sidecars. Video bytes never enter this directory.
func Sync(ctx context.Context, provider Provider, directory string, limit int) (int, error) {
	if limit < 1 || limit > 50 {
		return 0, errors.New("catalog limit must be between 1 and 50")
	}
	count := 0
	for _, id := range curatedIDs {
		if count == limit {
			break
		}
		if !safeID(id) {
			continue
		}
		detail, err := provider.GetMedia(ctx, id)
		if err != nil || detail == nil || detail.RightsStatus != media.RightsVerified {
			continue
		}
		sources, err := provider.GetSources(ctx, *detail)
		if err != nil {
			continue
		}
		for _, source := range sources {
			if !source.DirectPlay || source.RequiresProxy {
				continue
			}
			stream, err := provider.Resolve(ctx, source)
			if err != nil || stream == nil || stream.ProxyRequired || !archiveURL(stream.URL) {
				continue
			}
			if err := writeMovie(directory, id, *detail, stream.URL); err != nil {
				return count, err
			}
			count++
			break
		}
	}
	if count == 0 {
		return 0, errors.New("no licensed direct-play movie candidates found")
	}
	return count, nil
}

func safeID(id string) bool {
	if id == "" || len(id) > 128 || id == "." || id == ".." {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

func archiveURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "archive.org" && strings.HasPrefix(u.Path, "/download/") && u.User == nil
}

func writeMovie(directory, id string, detail media.Media, streamURL string) error {
	dir := filepath.Join(directory, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	nfo, err := xml.MarshalIndent(movieNFO{Title: detail.Title, Year: detail.Year, Plot: detail.Overview}, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(dir, "movie.nfo"), append([]byte(xml.Header), nfo...)); err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, "movie.strm"), []byte(streamURL+"\n"))
}

func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".catalog-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
