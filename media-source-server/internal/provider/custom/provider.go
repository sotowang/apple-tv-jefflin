package custom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

type Provider struct {
	base   *url.URL
	client *http.Client
	key    string
}
type StatusError struct {
	Code int
	Kind error
}

func (e *StatusError) Error() string       { return fmt.Sprintf("custom upstream status %d", e.Code) }
func (e *StatusError) Unwrap() error       { return e.Kind }
func (e *StatusError) UpstreamStatus() int { return e.Code }

type item struct {
	ProviderItemID string     `json:"providerItemId"`
	ExternalID     string     `json:"externalId"`
	Title          string     `json:"title"`
	OriginalTitle  string     `json:"originalTitle"`
	Year           int        `json:"year"`
	Season         int        `json:"season"`
	Episode        int        `json:"episode"`
	EpisodeTitle   string     `json:"episodeTitle"`
	URL            string     `json:"url"`
	Quality        string     `json:"quality"`
	Container      string     `json:"container"`
	Language       string     `json:"language"`
	DirectPlay     bool       `json:"directPlay"`
	RequiresProxy  bool       `json:"requiresProxy"`
	Width          int        `json:"width"`
	Height         int        `json:"height"`
	Bitrate        int64      `json:"bitrate"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	Ephemeral      bool       `json:"ephemeral"`
}

func New(raw, key string, timeout time.Duration, allowPrivate bool) (*Provider, error) {
	base, err := url.Parse(raw)
	if err != nil || base.Hostname() == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, errors.New("invalid custom source base URL")
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	if !allowPrivate && base.Scheme != "https" {
		return nil, errors.New("custom source requires HTTPS")
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if allowed(ip, allowPrivate) {
				return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			}
		}
		return nil, errors.New("custom source address blocked")
	}
	if !allowPrivate {
		lookupCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		ips, err := net.DefaultResolver.LookupNetIP(lookupCtx, "ip", base.Hostname())
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if !allowed(ip, false) {
				return nil, errors.New("custom source address blocked")
			}
		}
	}
	p := &Provider{base: base, key: key}
	p.client = &http.Client{Timeout: timeout, Transport: tr, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !strings.EqualFold(req.URL.Host, base.Host) || req.URL.Scheme != base.Scheme {
			return errors.New("custom source redirect blocked")
		}
		return nil
	}}
	return p, nil
}
func allowed(ip netip.Addr, private bool) bool {
	return private || (ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast())
}
func (p *Provider) Name() string  { return "custom" }
func (p *Provider) Enabled() bool { return p != nil }
func (p *Provider) ResolveMedia(ctx context.Context, m media.Media) ([]media.Source, error) {
	if m.Type != "movie" {
		return []media.Source{}, nil
	}
	q := url.Values{"title": {m.Title}}
	if m.Year > 0 {
		q.Set("year", strconv.Itoa(m.Year))
	}
	return p.fetch(ctx, m, nil, "/v1/search/movie", q)
}
func (p *Provider) ResolveEpisode(ctx context.Context, m media.Media, e media.Episode) ([]media.Source, error) {
	q := url.Values{"title": {m.Title}, "season": {strconv.Itoa(e.SeasonNumber)}, "episode": {strconv.Itoa(e.EpisodeNumber)}}
	return p.fetch(ctx, m, &e, "/v1/search/episode", q)
}
func (p *Provider) fetch(ctx context.Context, m media.Media, e *media.Episode, path string, q url.Values) ([]media.Source, error) {
	endpoint := *p.base
	endpoint.Path = strings.TrimRight(p.base.Path, "/") + path
	endpoint.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if p.key != "" {
		req.Header.Set("Authorization", "Bearer "+p.key)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", provider.ErrUpstreamUnavailable, err)
	}
	defer resp.Body.Close()
	media.SetUpstreamStatus(ctx, resp.StatusCode)
	switch resp.StatusCode {
	case 200:
	case 404:
		return []media.Source{}, nil
	case 401, 403:
		return nil, &StatusError{resp.StatusCode, provider.ErrUnauthorized}
	case 429:
		return nil, &StatusError{resp.StatusCode, provider.ErrRateLimited}
	default:
		return nil, &StatusError{resp.StatusCode, provider.ErrUpstreamUnavailable}
	}
	var payload struct {
		Items []item `json:"items"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("%w: %w", provider.ErrDecode, err)
	}
	out := make([]media.Source, 0, len(payload.Items))
	for _, it := range payload.Items {
		u, err := url.Parse(it.URL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			continue
		}
		if e != nil && (it.Season != e.SeasonNumber || it.Episode != e.EpisodeNumber) {
			continue
		}
		s := media.Source{MediaID: m.ID, Provider: "custom", ProviderItemID: it.ProviderItemID, ExternalID: it.ExternalID, Title: it.Title, OriginalTitle: it.OriginalTitle, Year: it.Year, URL: it.URL, OriginalURL: it.URL, Quality: it.Quality, Container: it.Container, Language: it.Language, DirectPlay: it.DirectPlay, RequiresProxy: it.RequiresProxy, Width: it.Width, Height: it.Height, Bitrate: it.Bitrate, ExpiresAt: it.ExpiresAt, Ephemeral: true}
		if e != nil {
			s.SeasonNumber = e.SeasonNumber
			s.EpisodeNumber = e.EpisodeNumber
		}
		out = append(out, s)
	}
	return out, nil
}
func (p *Provider) ResolveStream(_ context.Context, s media.Source) (*media.ResolvedStream, error) {
	if s.URL == "" {
		return nil, provider.ErrNotFound
	}
	return &media.ResolvedStream{URL: s.URL, ProxyRequired: s.RequiresProxy}, nil
}
