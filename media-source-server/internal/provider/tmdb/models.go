package tmdb

type result struct {
	ID               int         `json:"id"`
	MediaType        string      `json:"media_type"`
	Title            string      `json:"title"`
	OriginalTitle    string      `json:"original_title"`
	Name             string      `json:"name"`
	OriginalName     string      `json:"original_name"`
	Overview         string      `json:"overview"`
	ReleaseDate      string      `json:"release_date"`
	FirstAirDate     string      `json:"first_air_date"`
	PosterPath       string      `json:"poster_path"`
	BackdropPath     string      `json:"backdrop_path"`
	OriginalLanguage string      `json:"original_language"`
	Popularity       float64     `json:"popularity"`
	NumberOfSeasons  int         `json:"number_of_seasons"`
	NumberOfEpisodes int         `json:"number_of_episodes"`
	Seasons          []seasonDTO `json:"seasons"`
}
type seasonDTO struct {
	ID           int          `json:"id"`
	SeasonNumber int          `json:"season_number"`
	Name         string       `json:"name"`
	Overview     string       `json:"overview"`
	PosterPath   string       `json:"poster_path"`
	AirDate      string       `json:"air_date"`
	EpisodeCount int          `json:"episode_count"`
	Episodes     []episodeDTO `json:"episodes"`
}
type episodeDTO struct {
	ID            int    `json:"id"`
	EpisodeNumber int    `json:"episode_number"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	AirDate       string `json:"air_date"`
	Runtime       int    `json:"runtime"`
	StillPath     string `json:"still_path"`
}
