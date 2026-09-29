package media

import "context"

type Media struct {
	ID            string `json:"id"`
	Type          string `json:"type"`
	Title         string `json:"title"`
	OriginalTitle string `json:"originalTitle,omitempty"`
	Year          int    `json:"year,omitempty"`
	Overview      string `json:"overview,omitempty"`
	PosterURL     string `json:"posterUrl,omitempty"`
	BackdropURL   string `json:"backdropUrl,omitempty"`
	Provider      string `json:"provider"`
	ExternalID    string `json:"externalId"`
	LicenseURL    string `json:"licenseUrl,omitempty"`
	Rights        string `json:"rights,omitempty"`
}
type Source struct {
	ID            string `json:"id"`
	MediaID       string `json:"mediaId"`
	Provider      string `json:"provider"`
	ExternalID    string `json:"externalId,omitempty"`
	FileName      string `json:"fileName"`
	OriginalURL   string `json:"-"`
	Quality       string `json:"quality,omitempty"`
	Container     string `json:"container,omitempty"`
	VideoCodec    string `json:"videoCodec,omitempty"`
	AudioCodec    string `json:"audioCodec,omitempty"`
	Bitrate       int64  `json:"bitrate,omitempty"`
	DirectPlay    bool   `json:"directPlay"`
	RequiresProxy bool   `json:"requiresProxy"`
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
