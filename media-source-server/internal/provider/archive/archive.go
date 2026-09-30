package archive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"online-media/media-source-server/internal/cache"
	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

var ErrNotFound = errors.New("archive item not found")
var ErrNoPlayable = errors.New("no playable source")

type UpstreamStatusError struct{ StatusCode int }

func (e UpstreamStatusError) Error() string { return fmt.Sprintf("archive status %d", e.StatusCode) }

var htmlTagRE = regexp.MustCompile(`<[^>]*>`)
var identifierRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

func plain(v any) string {
	return strings.Join(strings.Fields(htmlTagRE.ReplaceAllString(value(v), " ")), " ")
}

type Provider struct {
	client        *resty.Client
	base          string
	metadataCache *cache.TTL[string, *metadata]
	metadataTTL   time.Duration
}

func New(base string, timeout, metadataTTL time.Duration) *Provider {
	return &Provider{
		client: resty.New().SetTimeout(timeout).SetResponseBodyLimit(4 << 20).SetRedirectPolicy(resty.FlexibleRedirectPolicy(2)),
		base:   strings.TrimRight(base, "/"), metadataCache: cache.New[string, *metadata](time.Minute), metadataTTL: metadataTTL,
	}
}
func (p *Provider) Name() string { return "archive" }
func (p *Provider) request(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	r, err := p.client.R().SetContext(ctx).SetQueryParams(params).Get(p.base + endpoint)
	if err != nil {
		return nil, err
	}
	media.SetUpstreamStatus(ctx, r.StatusCode())
	if r.StatusCode() == 404 {
		return nil, ErrNotFound
	}
	if r.StatusCode() != 200 {
		return nil, UpstreamStatusError{StatusCode: r.StatusCode()}
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
func (p *Provider) search(ctx context.Context, archiveQuery string, page, limit int, sortBy string) ([]media.Media, error) {
	params := map[string]string{"q": archiveQuery, "fl[]": "identifier,title,year,description", "rows": strconv.Itoa(limit), "page": strconv.Itoa(page), "output": "json"}
	if sortBy != "" {
		params["sort[]"] = sortBy
	}
	b, err := p.request(ctx, "/advancedsearch.php", params)
	if err != nil {
		return nil, err
	}
	var res searchResponse
	if err = json.Unmarshal(b, &res); err != nil {
		return nil, err
	}
	out := make([]media.Media, 0, len(res.Response.Docs))
	for _, d := range res.Response.Docs {
		if !identifierRE.MatchString(d.Identifier) {
			continue
		}
		out = append(out, media.Media{ID: "archive:" + d.Identifier, Type: "movie", Title: value(d.Title), Year: year(d.Year), Overview: plain(d.Description), Provider: "archive", ExternalID: d.Identifier})
	}
	return out, nil
}
func (p *Provider) Search(ctx context.Context, q media.SearchQuery) ([]media.Media, error) {
	escaped := strings.NewReplacer("\\", "\\\\", "\"", "\\\"").Replace(q.Query)
	return p.search(ctx, `mediatype:movies AND title:("`+escaped+`")`, q.Page, q.Limit, "")
}

// Featured is a browse query over the Archive open source movies collection.
// Collection membership is not used as a rights decision.
func (p *Provider) Featured(ctx context.Context, page, limit int) ([]media.Media, error) {
	return p.search(ctx, "mediatype:movies AND collection:opensource_movies AND format:MPEG4", page, limit, "downloads desc")
}

type archiveFile struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Format     string `json:"format"`
	Size       any    `json:"size"`
	Width      any    `json:"width"`
	Height     any    `json:"height"`
	VideoCodec string `json:"video_codec"`
	AudioCodec string `json:"audio_codec"`
}
type metadata struct {
	Metadata map[string]any `json:"metadata"`
	Files    []archiveFile  `json:"files"`
}

func (p *Provider) metadata(ctx context.Context, id string) (*metadata, error) {
	if !identifierRE.MatchString(id) {
		return nil, ErrNotFound
	}
	if cached, ok := p.metadataCache.Get(id); ok {
		return cached, nil
	}
	b, err := p.request(ctx, "/metadata/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	var m metadata
	if err = json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if len(m.Metadata) == 0 {
		return nil, ErrNotFound
	}
	p.metadataCache.Set(id, &m, p.metadataTTL)
	return &m, nil
}
func (p *Provider) GetMedia(ctx context.Context, id string) (*media.Media, error) {
	m, err := p.metadata(ctx, id)
	if err != nil {
		return nil, err
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
	result := &media.Media{ID: "archive:" + id, Type: "movie", Title: title, OriginalTitle: title, Year: year(m.Metadata["year"]), Overview: plain(m.Metadata["description"]), Provider: "archive", ExternalID: id, LicenseURL: license, Rights: rights, RightsStatus: media.ClassifyRights(license, rights)}
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
func playable(name, _ string) (string, bool) {
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
func containerRank(container string) int {
	switch container {
	case "mp4":
		return 0
	case "m4v":
		return 1
	case "mov":
		return 2
	case "webm":
		return 3
	case "mkv":
		return 4
	}
	return 5
}
func positiveInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 16384 {
		return 0
	}
	return n
}
func reasonableSize(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 || n > 200_000_000_000 {
		return 0
	}
	return n
}
func codecCandidate(container, video, audio string) bool {
	if containerRank(container) > 2 {
		return false
	}
	v, a := strings.ToLower(strings.TrimSpace(video)), strings.ToLower(strings.TrimSpace(audio))
	if v != "" && v != "h264" && v != "h.264" && v != "avc" && v != "avc1" {
		return false
	}
	if a != "" && a != "aac" && a != "mp4a" {
		return false
	}
	// Unknown codecs remain empty. This is only a container-level direct-play candidate.
	return true
}

type rankedSource struct {
	source          media.Source
	rank            int
	area            int
	size            int64
	original        bool
	formatPreferred bool
}

func (p *Provider) GetSources(ctx context.Context, m media.Media) ([]media.Source, error) {
	md, err := p.metadata(ctx, m.ExternalID)
	if err != nil {
		return nil, err
	}
	candidates := make([]rankedSource, 0, len(md.Files))
	for _, f := range md.Files {
		container, ok := playable(f.Name, f.Format)
		if !ok {
			continue
		}
		width, height := positiveInt(value(f.Width)), positiveInt(value(f.Height))
		quality := ""
		if height > 0 {
			quality = fmt.Sprintf("%dp", height)
		}
		// Archive's format label is only a tie breaker; it does not confirm a codec.
		candidates = append(candidates, rankedSource{source: media.Source{MediaID: m.ID, Provider: "archive", ExternalID: m.ExternalID, FileName: f.Name, Container: container, VideoCodec: strings.TrimSpace(f.VideoCodec), AudioCodec: strings.TrimSpace(f.AudioCodec), Quality: quality, DirectPlay: codecCandidate(container, f.VideoCodec, f.AudioCodec)}, rank: containerRank(container), area: width * height, size: reasonableSize(value(f.Size)), original: strings.EqualFold(f.Source, "original"), formatPreferred: strings.EqualFold(f.Format, "MPEG4")})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.source.DirectPlay != b.source.DirectPlay {
			return a.source.DirectPlay
		}
		if a.rank != b.rank {
			return a.rank < b.rank
		}
		if a.area != b.area {
			return a.area > b.area
		}
		if a.original != b.original {
			return a.original
		}
		if a.formatPreferred != b.formatPreferred {
			return a.formatPreferred
		}
		if a.size != b.size {
			return a.size > b.size
		}
		return a.source.FileName < b.source.FileName
	})
	if len(candidates) == 0 {
		return nil, ErrNoPlayable
	}
	out := make([]media.Source, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.source)
	}
	return out, nil
}
func (p *Provider) Resolve(_ context.Context, s media.Source) (*media.ResolvedStream, error) {
	if !identifierRE.MatchString(s.ExternalID) {
		return nil, ErrNotFound
	}
	if _, ok := playable(s.FileName, ""); !ok {
		return nil, ErrNoPlayable
	}
	return &media.ResolvedStream{URL: p.base + "/download/" + url.PathEscape(s.ExternalID) + "/" + url.PathEscape(s.FileName), ProxyRequired: false}, nil
}

func (p *Provider) GetSeasons(context.Context, string) ([]media.Season, error) {
	return []media.Season{}, nil
}
func (p *Provider) GetEpisodes(context.Context, string, int) ([]media.Episode, error) {
	return []media.Episode{}, nil
}
func (p *Provider) ResolveMedia(ctx context.Context, m media.Media) ([]media.Source, error) {
	if m.Provider != p.Name() {
		return nil, provider.ErrUnsupported
	}
	return p.GetSources(ctx, m)
}
func (p *Provider) ResolveEpisode(context.Context, media.Media, media.Episode) ([]media.Source, error) {
	return nil, provider.ErrUnsupported
}
func (p *Provider) ResolveStream(ctx context.Context, s media.Source) (*media.ResolvedStream, error) {
	return p.Resolve(ctx, s)
}
