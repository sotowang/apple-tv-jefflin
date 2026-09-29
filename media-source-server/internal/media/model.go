package media

import "context"

type Media struct {
	ID               string       `json:"id"`
	Type             string       `json:"type"`
	Title            string       `json:"title"`
	OriginalTitle    string       `json:"originalTitle,omitempty"`
	Year             int          `json:"year,omitempty"`
	Overview         string       `json:"overview,omitempty"`
	PosterURL        string       `json:"posterUrl,omitempty"`
	BackdropURL      string       `json:"backdropUrl,omitempty"`
	Provider         string       `json:"provider"`
	ExternalID       string       `json:"externalId"`
	ReleaseDate      string       `json:"releaseDate,omitempty"`
	OriginalLanguage string       `json:"originalLanguage,omitempty"`
	TMDBID           int          `json:"tmdbId,omitempty"`
	SeasonCount      int          `json:"seasonCount,omitempty"`
	EpisodeCount     int          `json:"episodeCount,omitempty"`
	LicenseURL       string       `json:"licenseUrl,omitempty"`
	Rights           string       `json:"rights,omitempty"`
	RightsStatus     RightsStatus `json:"rightsStatus,omitempty"`
}

type MediaType string

const (
	MediaTypeMovie MediaType = "movie"
	MediaTypeTV    MediaType = "tv"
)

type Season struct {
	ID           string `json:"id"`
	MediaID      string `json:"mediaId"`
	SeasonNumber int    `json:"seasonNumber"`
	Name         string `json:"name"`
	Overview     string `json:"overview,omitempty"`
	PosterURL    string `json:"posterUrl,omitempty"`
	AirDate      string `json:"airDate,omitempty"`
	EpisodeCount int    `json:"episodeCount"`
	ExternalID   string `json:"externalId,omitempty"`
}

type Episode struct {
	ID             string `json:"id"`
	SeasonID       string `json:"seasonId"`
	EpisodeNumber  int    `json:"episodeNumber"`
	Name           string `json:"name"`
	Overview       string `json:"overview,omitempty"`
	AirDate        string `json:"airDate,omitempty"`
	RuntimeMinutes int    `json:"runtimeMinutes,omitempty"`
	StillURL       string `json:"stillUrl,omitempty"`
	ExternalID     string `json:"externalId,omitempty"`
}
type Source struct {
	ID          string `json:"id"`
	MediaID     string `json:"mediaId"`
	Provider    string `json:"provider"`
	ExternalID  string `json:"externalId,omitempty"`
	FileName    string `json:"fileName"`
	OriginalURL string `json:"-"`
	Quality     string `json:"quality,omitempty"`
	Container   string `json:"container,omitempty"`
	VideoCodec  string `json:"videoCodec,omitempty"`
	AudioCodec  string `json:"audioCodec,omitempty"`
	Bitrate     int64  `json:"bitrate,omitempty"`
	// DirectPlay is a container-level direct-play candidate; codecs are not probed.
	DirectPlay    bool `json:"directPlay"`
	RequiresProxy bool `json:"requiresProxy"`
}
type ResolvedStream struct {
	URL           string
	Headers       map[string]string
	ProxyRequired bool
}
type SearchQuery struct {
	Query       string
	Page, Limit int
}
type Provider interface {
	Name() string
	Search(context.Context, SearchQuery) ([]Media, error)
	GetMedia(context.Context, string) (*Media, error)
	GetSources(context.Context, Media) ([]Source, error)
	Resolve(context.Context, Source) (*ResolvedStream, error)
}

// BrowseProvider is optional; browsing never reuses a text-search query.
type BrowseProvider interface {
	Featured(context.Context, int, int) ([]Media, error)
}
