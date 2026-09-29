package api

import "online-media/media-source-server/internal/media"

// API records are kept separate from provider payloads and persisted domain models.
type mediaRecord struct {
	ID               string             `json:"id"`
	Type             string             `json:"type"`
	Title            string             `json:"title"`
	OriginalTitle    string             `json:"originalTitle,omitempty"`
	Year             int                `json:"year,omitempty"`
	Overview         string             `json:"overview,omitempty"`
	PosterURL        string             `json:"posterUrl,omitempty"`
	BackdropURL      string             `json:"backdropUrl,omitempty"`
	Provider         string             `json:"provider"`
	ExternalID       string             `json:"externalId"`
	ReleaseDate      string             `json:"releaseDate,omitempty"`
	OriginalLanguage string             `json:"originalLanguage,omitempty"`
	TMDBID           int                `json:"tmdbId,omitempty"`
	SeasonCount      int                `json:"seasonCount,omitempty"`
	EpisodeCount     int                `json:"episodeCount,omitempty"`
	LicenseURL       string             `json:"licenseUrl,omitempty"`
	Rights           string             `json:"rights,omitempty"`
	RightsStatus     media.RightsStatus `json:"rightsStatus,omitempty"`
}

func toMediaRecord(m media.Media) mediaRecord {
	return mediaRecord{ID: m.ID, Type: m.Type, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: m.Year, Overview: m.Overview, PosterURL: m.PosterURL, BackdropURL: m.BackdropURL, Provider: m.Provider, ExternalID: m.ExternalID, ReleaseDate: m.ReleaseDate, OriginalLanguage: m.OriginalLanguage, TMDBID: m.TMDBID, SeasonCount: m.SeasonCount, EpisodeCount: m.EpisodeCount, LicenseURL: m.LicenseURL, Rights: m.Rights, RightsStatus: m.RightsStatus}
}
func toMediaRecords(items []media.Media) []mediaRecord {
	out := make([]mediaRecord, 0, len(items))
	for _, m := range items {
		out = append(out, toMediaRecord(m))
	}
	return out
}

type seasonRecord struct {
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

func toSeasonRecords(items []media.Season) []seasonRecord {
	out := make([]seasonRecord, 0, len(items))
	for _, s := range items {
		out = append(out, seasonRecord{ID: s.ID, MediaID: s.MediaID, SeasonNumber: s.SeasonNumber, Name: s.Name, Overview: s.Overview, PosterURL: s.PosterURL, AirDate: s.AirDate, EpisodeCount: s.EpisodeCount, ExternalID: s.ExternalID})
	}
	return out
}

type episodeRecord struct {
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

func toEpisodeRecords(items []media.Episode) []episodeRecord {
	out := make([]episodeRecord, 0, len(items))
	for _, e := range items {
		out = append(out, episodeRecord{ID: e.ID, SeasonID: e.SeasonID, EpisodeNumber: e.EpisodeNumber, Name: e.Name, Overview: e.Overview, AirDate: e.AirDate, RuntimeMinutes: e.RuntimeMinutes, StillURL: e.StillURL, ExternalID: e.ExternalID})
	}
	return out
}
