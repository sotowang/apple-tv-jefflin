package provider

import (
	"context"
	"errors"
	"sort"

	"online-media/media-source-server/internal/media"
)

var ErrNotFound = errors.New("media not found")
var ErrUnsupported = errors.New("operation unsupported")
var ErrUnauthorized = errors.New("provider unauthorized")
var ErrRateLimited = errors.New("provider rate limited")
var ErrUpstreamUnavailable = errors.New("provider upstream unavailable")
var ErrDecode = errors.New("provider response decode error")

type MetadataProvider interface {
	Name() string
	Search(context.Context, media.SearchQuery) ([]media.Media, error)
	GetMedia(context.Context, string) (*media.Media, error)
	GetSeasons(context.Context, string) ([]media.Season, error)
	GetEpisodes(context.Context, string, int) ([]media.Episode, error)
}

type SourceProvider interface {
	Name() string
	ResolveMedia(context.Context, media.Media) ([]media.Source, error)
	ResolveEpisode(context.Context, media.Media, media.Episode) ([]media.Source, error)
	ResolveStream(context.Context, media.Source) (*media.ResolvedStream, error)
}

type Registry struct {
	MetadataProviders map[string]MetadataProvider
	SourceProviders   map[string]SourceProvider
	BrowseProviders   map[string]media.BrowseProvider
}

func NewRegistry() *Registry {
	return &Registry{MetadataProviders: map[string]MetadataProvider{}, SourceProviders: map[string]SourceProvider{}, BrowseProviders: map[string]media.BrowseProvider{}}
}
func (r *Registry) RegisterMetadata(p MetadataProvider)                { r.MetadataProviders[p.Name()] = p }
func (r *Registry) RegisterSource(p SourceProvider)                    { r.SourceProviders[p.Name()] = p }
func (r *Registry) RegisterBrowse(name string, p media.BrowseProvider) { r.BrowseProviders[name] = p }
func (r *Registry) MetadataNames() []string {
	names := make([]string, 0, len(r.MetadataProviders))
	for name := range r.MetadataProviders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveMedia asks every registered source provider. A metadata-only item has no sources.
func (r *Registry) ResolveMedia(ctx context.Context, m media.Media) ([]media.Source, error) {
	out := []media.Source{}
	for _, name := range sourceNames(r.SourceProviders) {
		found, err := r.SourceProviders[name].ResolveMedia(ctx, m)
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}
func (r *Registry) ResolveEpisode(ctx context.Context, m media.Media, e media.Episode) ([]media.Source, error) {
	out := []media.Source{}
	for _, name := range sourceNames(r.SourceProviders) {
		found, err := r.SourceProviders[name].ResolveEpisode(ctx, m, e)
		if err != nil {
			return nil, err
		}
		out = append(out, found...)
	}
	return out, nil
}
func sourceNames(providers map[string]SourceProvider) []string {
	names := make([]string, 0, len(providers))
	for name := range providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type legacyMetadata struct{ media.Provider }

func (legacyMetadata) GetSeasons(context.Context, string) ([]media.Season, error) {
	return []media.Season{}, nil
}
func (legacyMetadata) GetEpisodes(context.Context, string, int) ([]media.Episode, error) {
	return []media.Episode{}, nil
}

type legacySource struct{ media.Provider }

func (p legacySource) ResolveMedia(ctx context.Context, m media.Media) ([]media.Source, error) {
	if m.Provider != p.Name() {
		return []media.Source{}, nil
	}
	return p.GetSources(ctx, m)
}
func (legacySource) ResolveEpisode(context.Context, media.Media, media.Episode) ([]media.Source, error) {
	return []media.Source{}, nil
}
func (p legacySource) ResolveStream(ctx context.Context, s media.Source) (*media.ResolvedStream, error) {
	return p.Resolve(ctx, s)
}
func (r *Registry) RegisterLegacy(p media.Provider) {
	r.RegisterMetadata(legacyMetadata{p})
	r.RegisterSource(legacySource{p})
	if b, ok := p.(media.BrowseProvider); ok {
		r.RegisterBrowse(p.Name(), b)
	}
}
