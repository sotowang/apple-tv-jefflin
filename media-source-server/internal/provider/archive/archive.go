package archive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"net/url"
	"online-media/media-source-server/internal/cache"
	"online-media/media-source-server/internal/media"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrNotFound = errors.New("archive item not found")
var ErrNoPlayable = errors.New("no playable source")
var htmlTagRE = regexp.MustCompile(`<[^>]*>`)

func plain(v any) string {
	return strings.Join(strings.Fields(htmlTagRE.ReplaceAllString(value(v), " ")), " ")
}

var identifierRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

type Provider struct {
	client        *resty.Client
	base          string
	metadataCache *cache.TTL[string, *metadata]
	metadataTTL   time.Duration
}

func New(base string, timeout, metadataTTL time.Duration) *Provider {
	return &Provider{client: resty.New().SetTimeout(timeout).SetResponseBodyLimit(4 << 20).SetRedirectPolicy(resty.FlexibleRedirectPolicy(2)), base: strings.TrimRight(base, "/"), metadataCache: cache.New[string, *metadata](time.Minute), metadataTTL: metadataTTL}
}
func (p *Provider) Name() string { return "archive" }
func (p *Provider) request(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	r, err := p.client.R().SetContext(ctx).SetQueryParams(params).Get(p.base + endpoint)
	if err != nil {
		return nil, err
	}
	if r.StatusCode() == 404 {
		return nil, ErrNotFound
	}
	if r.StatusCode() != 200 {
		return nil, fmt.Errorf("archive status %d", r.StatusCode())
	}
	return r.Body(), nil
}

type searchResponse struct {
	Response struct {
		Docs []struct {
			Identifier  string `json:"identifier"`
			Title       any    `json:"title"`
			Year        any    `json:"year"`
			Description any    `json:"description"`
		} `json:"docs"`
	} `json:"response"`
}

func value(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		if len(x) > 0 {
			return value(x[0])
		}
	case float64:
		return strconv.Itoa(int(x))
	}
	return ""
}
func year(v any) int {
	s := value(v)
	if len(s) >= 4 {
		n, _ := strconv.Atoi(s[:4])
		if n >= 1800 && n <= 2100 {
			return n
		}
	}
	return 0
}
func (p *Provider) Search(ctx context.Context, q media.SearchQuery) ([]media.Media, error) {
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(q.Query)
	b, e := p.request(ctx, "/advancedsearch.php", map[string]string{"q": `mediatype:movies AND title:("` + escaped + `")`, "fl[]": "identifier,title,year,description", "rows": strconv.Itoa(q.Limit), "page": strconv.Itoa(q.Page), "output": "json"})
	if e != nil {
		return nil, e
	}
	var res searchResponse
	if e = json.Unmarshal(b, &res); e != nil {
		return nil, e
	}
	out := make([]media.Media, 0, len(res.Response.Docs))
	for _, d := range res.Response.Docs {
		if !identifierRE.MatchString(d.Identifier) {
			continue
		}
		out = append(out, media.Media{ID: "archive:" + d.Identifier, Type: "movie", Title: value(d.Title), Year: year(d.Year), Overview: value(d.Description), Provider: "archive", ExternalID: d.Identifier})
	}
	return out, nil
}

type metadata struct {
	Metadata map[string]any `json:"metadata"`
	Files    []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
		Format string `json:"format"`
		Size   string `json:"size"`
		Length string `json:"length"`
		Width  string `json:"width"`
		Height string `json:"height"`
	} `json:"files"`
}

func (p *Provider) metadata(ctx context.Context, id string) (*metadata, error) {
	if !identifierRE.MatchString(id) {
		return nil, ErrNotFound
	}
	if cached, ok := p.metadataCache.Get(id); ok {
		return cached, nil
	}
	b, e := p.request(ctx, "/metadata/"+url.PathEscape(id), nil)
	if e != nil {
		return nil, e
	}
	var m metadata
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, e
	}
	if len(m.Metadata) == 0 {
		return nil, ErrNotFound
	}
	p.metadataCache.Set(id, &m, p.metadataTTL)
	return &m, nil
}
func (p *Provider) GetMedia(ctx context.Context, id string) (*media.Media, error) {
	m, e := p.metadata(ctx, id)
	if e != nil {
		return nil, e
	}
	title := value(m.Metadata["title"])
	if title == "" {
		title = id
	}
	license := value(m.Metadata["licenseurl"])
	rights := value(m.Metadata["rights"])
	collection := value(m.Metadata["collection"])
	if collection != "" {
		if rights != "" {
			rights += "; "
		}
		rights += "collection: " + collection
	}
	result := &media.Media{ID: "archive:" + id, Type: "movie", Title: title, OriginalTitle: title, Year: year(m.Metadata["year"]), Overview: value(m.Metadata["description"]), Provider: "archive", ExternalID: id, LicenseURL: license, Rights: rights}
	if result.Year == 0 {
		result.Year = year(m.Metadata["date"])
	}
	for _, f := range m.Files {
		if strings.EqualFold(f.Name, "__ia_thumb.jpg") {
			result.PosterURL = p.base + "/services/img/" + url.PathEscape(id)
			break
		}
	}
	return result, nil
}
func playable(name, format string) (string, bool) {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "/") || strings.Contains(lower, "\\") || strings.HasPrefix(lower, ".") {
		return "", false
	}
	switch strings.ToLower(path.Ext(lower)) {
	case ".mp4":
		return "mp4", true
	case ".m4v":
		return "m4v", true
	case ".mov":
		return "mov", true
	case ".webm":
		return "webm", true
	case ".mkv":
		return "mkv", true
	}
	return "", false
}
func (p *Provider) GetSources(ctx context.Context, m media.Media) ([]media.Source, error) {
	md, e := p.metadata(ctx, m.ExternalID)
	if e != nil {
		return nil, e
	}
	out := []media.Source{}
	for _, f := range md.Files {
		container, ok := playable(f.Name, f.Format)
		if !ok {
			continue
		}
		direct := container == "mp4" || container == "m4v" || container == "mov"
		out = append(out, media.Source{MediaID: m.ID, Provider: "archive", ExternalID: m.ExternalID, FileName: f.Name, Container: container, DirectPlay: direct})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DirectPlay != out[j].DirectPlay {
			return out[i].DirectPlay
		}
		return out[i].FileName < out[j].FileName
	})
	if len(out) == 0 {
		return nil, ErrNoPlayable
	}
	return out, nil
}
func (p *Provider) Resolve(ctx context.Context, s media.Source) (*media.ResolvedStream, error) {
	if !identifierRE.MatchString(s.ExternalID) {
		return nil, ErrNotFound
	}
	if _, ok := playable(s.FileName, ""); !ok {
		return nil, ErrNoPlayable
	}
	return &media.ResolvedStream{URL: p.base + "/download/" + url.PathEscape(s.ExternalID) + "/" + url.PathEscape(s.FileName), ProxyRequired: false}, nil
}
