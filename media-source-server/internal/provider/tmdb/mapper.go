package tmdb

import (
	"online-media/media-source-server/internal/media"
	"strconv"
	"strings"
)

func imageURL(size, path string) string {
	if path == "" || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return ""
	}
	return "https://image.tmdb.org/t/p/" + size + path
}
func year(date string) int {
	if len(date) < 4 {
		return 0
	}
	n, err := strconv.Atoi(date[:4])
	if err != nil || n < 1800 || n > 2200 {
		return 0
	}
	return n
}
func mapMedia(d result, kind string) media.Media {
	title, original, date := d.Title, d.OriginalTitle, d.ReleaseDate
	if kind == "tv" {
		title, original, date = d.Name, d.OriginalName, d.FirstAirDate
	}
	id := strconv.Itoa(d.ID)
	return media.Media{ID: "tmdb:" + kind + ":" + id, Provider: "tmdb", ExternalID: kind + ":" + id, Type: kind,
		Title: title, OriginalTitle: original, Overview: d.Overview, ReleaseDate: date, Year: year(date),
		PosterURL: imageURL("w500", d.PosterPath), BackdropURL: imageURL("w1280", d.BackdropPath),
		OriginalLanguage: d.OriginalLanguage, TMDBID: d.ID, SeasonCount: d.NumberOfSeasons, EpisodeCount: d.NumberOfEpisodes}
}
func mapSeason(mediaID string, d seasonDTO) media.Season {
	id := strconv.Itoa(d.ID)
	return media.Season{ID: mediaID + ":season:" + strconv.Itoa(d.SeasonNumber), MediaID: mediaID, SeasonNumber: d.SeasonNumber,
		Name: d.Name, Overview: d.Overview, PosterURL: imageURL("w500", d.PosterPath), AirDate: d.AirDate,
		EpisodeCount: d.EpisodeCount, ExternalID: id}
}
func mapEpisode(seasonID string, d episodeDTO) media.Episode {
	return media.Episode{ID: seasonID + ":episode:" + strconv.Itoa(d.EpisodeNumber), SeasonID: seasonID,
		EpisodeNumber: d.EpisodeNumber, Name: d.Name, Overview: d.Overview, AirDate: d.AirDate,
		RuntimeMinutes: d.Runtime, StillURL: imageURL("w780", d.StillPath), ExternalID: strconv.Itoa(d.ID)}
}
