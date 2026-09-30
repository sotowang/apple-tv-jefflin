package source

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

type Resolver struct {
	providers []provider.SourceProvider
	priority  map[string]int
}

func New(reg *provider.Registry, order string) *Resolver {
	if order == "" {
		order = "custom,archive"
	}
	r := &Resolver{priority: map[string]int{}}
	for i, name := range strings.Split(order, ",") {
		r.priority[strings.TrimSpace(name)] = i
	}
	for _, p := range reg.SourceProviders {
		if enabled, ok := p.(interface{ Enabled() bool }); ok && !enabled.Enabled() {
			continue
		}
		r.providers = append(r.providers, p)
	}
	sort.Slice(r.providers, func(i, j int) bool {
		a, b := r.providers[i].Name(), r.providers[j].Name()
		if r.rank(a) != r.rank(b) {
			return r.rank(a) < r.rank(b)
		}
		return a < b
	})
	return r
}
func (r *Resolver) rank(name string) int {
	if n, ok := r.priority[name]; ok {
		return n
	}
	return len(r.priority) + 100
}
func (r *Resolver) ResolveMedia(ctx context.Context, m media.Media) ([]media.Source, error) {
	return r.resolve(ctx, m, nil)
}
func (r *Resolver) ResolveEpisode(ctx context.Context, m media.Media, e media.Episode) ([]media.Source, error) {
	return r.resolve(ctx, m, &e)
}
func (r *Resolver) resolve(ctx context.Context, m media.Media, e *media.Episode) ([]media.Source, error) {
	out := []media.Source{}
	var firstErr error
	success := false
	operation := "media"
	season, episode := 0, 0
	if e != nil {
		operation = "episode"
		season, episode = e.SeasonNumber, e.EpisodeNumber
	}
	for _, p := range r.providers {
		start := time.Now()
		var found []media.Source
		var err error
		if e == nil {
			found, err = p.ResolveMedia(ctx, m)
		} else {
			found, err = p.ResolveEpisode(ctx, m, *e)
		}
		status := "ok"
		if err != nil {
			status = "error"
			if errors.Is(err, provider.ErrUnsupported) || errors.Is(err, provider.ErrNotFound) {
				status = "no_result"
			} else if errors.Is(err, provider.ErrUnauthorized) {
				status = "auth_error"
			} else if errors.Is(err, provider.ErrRateLimited) {
				status = "rate_limited"
			} else if errors.Is(err, provider.ErrDecode) {
				status = "decode_error"
			} else if errors.Is(err, context.DeadlineExceeded) {
				status = "timeout"
			} else if firstErr == nil {
				firstErr = err
			}
			if status != "no_result" && firstErr == nil {
				firstErr = err
			}
		} else {
			success = true
		}
		upstream := 0
		var se interface{ UpstreamStatus() int }
		if errors.As(err, &se) {
			upstream = se.UpstreamStatus()
		}
		slog.Info("source_provider", "provider", p.Name(), "operation", operation, "mediaId", m.ID, "season", season, "episode", episode, "durationMs", time.Since(start).Milliseconds(), "resultCount", len(found), "status", status, "upstreamStatus", upstream)
		if err != nil {
			continue
		}
		for _, s := range found {
			if matches(m, e, s) {
				out = append(out, s)
			}
		}
	}
	if !success && firstErr != nil {
		return nil, firstErr
	}
	return r.Organize(out), nil
}
func (r *Resolver) Organize(out []media.Source) []media.Source {
	sort.SliceStable(out, func(i, j int) bool { return r.better(out[i], out[j]) })
	seenURL, seenID := map[string]bool{}, map[string]bool{}
	dedup := make([]media.Source, 0, len(out))
	for _, s := range out {
		u := normalizedURL(s.URL)
		if u == "" {
			u = normalizedURL(s.OriginalURL)
		}
		id := s.Provider + ":" + s.ProviderItemID
		if u != "" && seenURL[u] || s.ProviderItemID != "" && seenID[id] {
			continue
		}
		if u != "" {
			seenURL[u] = true
		}
		if s.ProviderItemID != "" {
			seenID[id] = true
		}
		dedup = append(dedup, s)
	}
	return dedup
}
func (r *Resolver) better(a, b media.Source) bool {
	if a.DirectPlay != b.DirectPlay {
		return a.DirectPlay
	}
	if a.RequiresProxy != b.RequiresProxy {
		return !a.RequiresProxy
	}
	if q(a) != q(b) {
		return q(a) > q(b)
	}
	az, bz := strings.EqualFold(a.Language, "zh-CN"), strings.EqualFold(b.Language, "zh-CN")
	if az != bz {
		return az
	}
	return r.rank(a.Provider) < r.rank(b.Provider)
}
func q(s media.Source) int {
	if s.Height > 0 {
		return s.Height
	}
	n, _ := strconv.Atoi(strings.TrimSuffix(strings.ToLower(s.Quality), "p"))
	return n
}
