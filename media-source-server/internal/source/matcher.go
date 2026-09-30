package source

import (
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"online-media/media-source-server/internal/media"
)

func normalizeTitle(s string) string {
	s = strings.ToLower(norm.NFKC.String(strings.TrimSpace(s)))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func matches(m media.Media, e *media.Episode, s media.Source) bool {
	if e != nil && (s.SeasonNumber != e.SeasonNumber || s.EpisodeNumber != e.EpisodeNumber) {
		return false
	}
	if (s.ExternalID != "" && (s.ExternalID == m.ExternalID || s.ExternalID == m.ID)) || (s.ProviderItemID != "" && (s.ProviderItemID == m.ExternalID || s.ProviderItemID == m.ID)) {
		return true
	}
	title := normalizeTitle(s.Title)
	if title == normalizeTitle(m.Title) || (m.OriginalTitle != "" && title == normalizeTitle(m.OriginalTitle)) {
		return s.Year == 0 || m.Year == 0 || s.Year == m.Year
	}
	original := normalizeTitle(s.OriginalTitle)
	if original != "" && (original == normalizeTitle(m.Title) || (m.OriginalTitle != "" && original == normalizeTitle(m.OriginalTitle))) {
		return s.Year == 0 || m.Year == 0 || s.Year == m.Year
	}
	if title == "" && original == "" {
		return s.Provider == m.Provider
	}
	return false
}

func normalizedURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	return u.String()
}
