package tmdb

import (
	"context"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"online-media/media-source-server/internal/media"
)

func (c *Client) Search(ctx context.Context, q media.SearchQuery) ([]media.Media, error) {
	// TMDB pages are fixed at 20. Sort the requested window and a small overlap.
	start := (q.Page - 1) * q.Limit
	first := start/20 + 1
	last := (start+q.Limit-1)/20 + 1
	fetchStart := 1
	if last > 10 {
		fetchStart = first
	}
	fetchEnd := last + 1
	if fetchEnd > 500 {
		fetchEnd = 500
	}
	all := make([]result, 0, (fetchEnd-fetchStart+1)*20)
	for page := fetchStart; page <= fetchEnd; page++ {
		params := url.Values{"query": {q.Query}, "page": {strconv.Itoa(page)}, "region": {c.Region}, "include_adult": {"false"}}
		var response struct {
			Results    []result `json:"results"`
			TotalPages int      `json:"total_pages"`
		}
		if err := c.get(ctx, "/search/multi", params, &response); err != nil {
			return nil, err
		}
		all = append(all, response.Results...)
		if page >= response.TotalPages {
			break
		}
	}
	filtered := make([]result, 0, len(all))
	for _, d := range all {
		if d.ID > 0 && (d.MediaType == "tv" || d.MediaType == "movie") {
			filtered = append(filtered, d)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := searchRank(q.Query, filtered[i]), searchRank(q.Query, filtered[j])
		if a != b {
			return a < b
		}
		if filtered[i].Popularity != filtered[j].Popularity {
			return filtered[i].Popularity > filtered[j].Popularity
		}
		return filtered[i].ID < filtered[j].ID
	})
	localStart := start - (fetchStart-1)*20
	if localStart >= len(filtered) {
		return []media.Media{}, nil
	}
	end := localStart + q.Limit
	if end > len(filtered) {
		end = len(filtered)
	}
	out := make([]media.Media, 0, end-localStart)
	for _, d := range filtered[localStart:end] {
		out = append(out, mapMedia(d, d.MediaType))
	}
	return out, nil
}
func normalized(s string) string {
	return strings.ToLower(strings.TrimFunc(strings.TrimSpace(s), unicode.IsSpace))
}
func searchRank(query string, d result) int {
	q := normalized(query)
	title, original := d.Title, d.OriginalTitle
	if d.MediaType == "tv" {
		title, original = d.Name, d.OriginalName
	}
	t, o := normalized(title), normalized(original)
	if t == q {
		return 0
	}
	if o == q {
		return 1
	}
	if strings.HasPrefix(t, q) {
		return 2
	}
	if strings.Contains(t, q) || strings.HasPrefix(o, q) {
		return 3
	}
	if strings.Contains(o, q) {
		return 4
	}
	return 5
}
