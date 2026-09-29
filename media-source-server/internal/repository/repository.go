package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"online-media/media-source-server/internal/database/dbgen"
	"online-media/media-source-server/internal/media"
	"time"
)

type Repository struct {
	Pool *pgxpool.Pool
	Q    *dbgen.Queries
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{Pool: pool, Q: dbgen.New(pool)} }
func UUID(s string) (pgtype.UUID, error) { var u pgtype.UUID; err := u.Scan(s); return u, err }
func ID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func convertMedia(m dbgen.Medium, provider string) media.Media {
	y := 0
	if m.Year.Valid {
		y = int(m.Year.Int32)
	}
	tmdbID := 0
	if m.TmdbID.Valid {
		tmdbID = int(m.TmdbID.Int32)
	}
	return media.Media{ID: provider + ":" + m.ExternalID, Type: m.Type, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: y, Overview: m.Overview, PosterURL: m.PosterUrl, BackdropURL: m.BackdropUrl, Provider: provider, ExternalID: m.ExternalID, LicenseURL: m.LicenseUrl, Rights: m.Rights, RightsStatus: media.ClassifyRights(m.LicenseUrl, m.Rights), ReleaseDate: dateString(m.ReleaseDate), OriginalLanguage: m.OriginalLanguage, TMDBID: tmdbID, SeasonCount: int(m.SeasonCount), EpisodeCount: int(m.EpisodeCount)}
}
func dateString(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format("2006-01-02")
}
func dateValue(s string) pgtype.Date {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}
func convertSource(s dbgen.Source, provider string) media.Source {
	b := int64(0)
	if s.Bitrate.Valid {
		b = s.Bitrate.Int64
	}
	return media.Source{ID: ID(s.ID), MediaID: ID(s.MediaID), Provider: provider, ExternalID: s.ExternalID, FileName: s.FileName, OriginalURL: s.OriginalUrl, Quality: s.Quality, Container: s.Container, VideoCodec: s.VideoCodec, AudioCodec: s.AudioCodec, Bitrate: b, DirectPlay: s.DirectPlay, RequiresProxy: s.RequiresProxy}
}
func (r *Repository) Provider(ctx context.Context, name string) (dbgen.Provider, error) {
	return r.Q.GetProviderByName(ctx, name)
}
func (r *Repository) Providers(ctx context.Context) ([]dbgen.Provider, error) {
	return r.Q.ListProviders(ctx)
}
func (r *Repository) MediaByExternal(ctx context.Context, provider, id string) (media.Media, error) {
	m, e := r.Q.GetMediaByExternalID(ctx, dbgen.GetMediaByExternalIDParams{Name: provider, ExternalID: id})
	return convertMedia(m, provider), e
}
func (r *Repository) UpsertMedia(ctx context.Context, m media.Media) (media.Media, error) {
	p, e := r.Provider(ctx, m.Provider)
	if e != nil {
		return media.Media{}, e
	}
	var yr pgtype.Int4
	if m.Year > 0 {
		yr = pgtype.Int4{Int32: int32(m.Year), Valid: true}
	}
	var tmdbID pgtype.Int4
	if m.TMDBID > 0 {
		tmdbID = pgtype.Int4{Int32: int32(m.TMDBID), Valid: true}
	}
	row, e := r.Q.UpsertMedia(ctx, dbgen.UpsertMediaParams{Type: m.Type, Title: m.Title, OriginalTitle: m.OriginalTitle, Year: yr, Overview: m.Overview, PosterUrl: m.PosterURL, BackdropUrl: m.BackdropURL, ProviderID: p.ID, ExternalID: m.ExternalID, LicenseUrl: m.LicenseURL, Rights: m.Rights, ReleaseDate: dateValue(m.ReleaseDate), OriginalLanguage: m.OriginalLanguage, TmdbID: tmdbID, SeasonCount: int32(m.SeasonCount), EpisodeCount: int32(m.EpisodeCount)})
	return convertMedia(row, m.Provider), e
}
func (r *Repository) MediaUUID(ctx context.Context, provider, id string) (pgtype.UUID, error) {
	row, e := r.Q.GetMediaByExternalID(ctx, dbgen.GetMediaByExternalIDParams{Name: provider, ExternalID: id})
	return row.ID, e
}
func (r *Repository) Sources(ctx context.Context, provider, id string) ([]media.Source, error) {
	uid, e := r.MediaUUID(ctx, provider, id)
	if e != nil {
		return nil, e
	}
	rows, e := r.Q.GetSourcesByMediaID(ctx, uid)
	if e != nil {
		return nil, e
	}
	out := make([]media.Source, 0, len(rows))
	for _, s := range rows {
		p, err := r.Q.GetProviderByID(ctx, s.ProviderID)
		if err != nil {
			return nil, err
		}
		out = append(out, convertSource(s, p.Name))
	}
	return out, nil
}
func (r *Repository) UpsertSources(ctx context.Context, m media.Media, sources []media.Source) ([]media.Source, error) {
	mid, e := r.MediaUUID(ctx, m.Provider, m.ExternalID)
	if e != nil {
		return nil, e
	}
	out := make([]media.Source, 0, len(sources))
	for _, s := range sources {
		p, e := r.Provider(ctx, s.Provider)
		if e != nil {
			return nil, e
		}
		var bitrate pgtype.Int8
		if s.Bitrate > 0 {
			bitrate = pgtype.Int8{Int64: s.Bitrate, Valid: true}
		}
		row, e := r.Q.UpsertSource(ctx, dbgen.UpsertSourceParams{MediaID: mid, ProviderID: p.ID, ExternalID: s.ExternalID, FileName: s.FileName, OriginalUrl: s.OriginalURL, Quality: s.Quality, Container: s.Container, VideoCodec: s.VideoCodec, AudioCodec: s.AudioCodec, Bitrate: bitrate, DirectPlay: s.DirectPlay, RequiresProxy: s.RequiresProxy})
		if e != nil {
			return nil, e
		}
		out = append(out, convertSource(row, m.Provider))
	}
	return out, nil
}

func convertSeason(s dbgen.Season, mediaID string) media.Season {
	return media.Season{ID: mediaID + ":season:" + fmt.Sprint(s.SeasonNumber), MediaID: mediaID, SeasonNumber: int(s.SeasonNumber), Name: s.Name, Overview: s.Overview, PosterURL: s.PosterUrl, AirDate: dateString(s.AirDate), EpisodeCount: int(s.EpisodeCount), ExternalID: s.ExternalID}
}
func convertEpisode(e dbgen.Episode, seasonID string) media.Episode {
	return media.Episode{ID: seasonID + ":episode:" + fmt.Sprint(e.EpisodeNumber), SeasonID: seasonID, EpisodeNumber: int(e.EpisodeNumber), Name: e.Name, Overview: e.Overview, AirDate: dateString(e.AirDate), RuntimeMinutes: int(e.RuntimeMinutes), StillURL: e.StillUrl, ExternalID: e.ExternalID}
}
func (r *Repository) UpsertSeasons(ctx context.Context, m media.Media, seasons []media.Season) ([]media.Season, error) {
	mid, err := r.MediaUUID(ctx, m.Provider, m.ExternalID)
	if err != nil {
		return nil, err
	}
	out := make([]media.Season, 0, len(seasons))
	for _, s := range seasons {
		row, err := r.Q.UpsertSeason(ctx, dbgen.UpsertSeasonParams{MediaID: mid, SeasonNumber: int32(s.SeasonNumber), Name: s.Name, Overview: s.Overview, PosterUrl: s.PosterURL, AirDate: dateValue(s.AirDate), EpisodeCount: int32(s.EpisodeCount), ExternalID: s.ExternalID})
		if err != nil {
			return nil, err
		}
		out = append(out, convertSeason(row, m.ID))
	}
	return out, nil
}
func (r *Repository) Seasons(ctx context.Context, m media.Media) ([]media.Season, error) {
	mid, err := r.MediaUUID(ctx, m.Provider, m.ExternalID)
	if err != nil {
		return nil, err
	}
	rows, err := r.Q.GetSeasonsByMediaID(ctx, mid)
	if err != nil {
		return nil, err
	}
	out := make([]media.Season, 0, len(rows))
	for _, row := range rows {
		out = append(out, convertSeason(row, m.ID))
	}
	return out, nil
}
func (r *Repository) UpsertEpisodes(ctx context.Context, m media.Media, number int, episodes []media.Episode) ([]media.Episode, error) {
	mid, err := r.MediaUUID(ctx, m.Provider, m.ExternalID)
	if err != nil {
		return nil, err
	}
	season, err := r.Q.GetSeasonByMediaAndNumber(ctx, dbgen.GetSeasonByMediaAndNumberParams{MediaID: mid, SeasonNumber: int32(number)})
	if err != nil {
		return nil, err
	}
	seasonID := m.ID + ":season:" + fmt.Sprint(number)
	out := make([]media.Episode, 0, len(episodes))
	for _, e := range episodes {
		row, err := r.Q.UpsertEpisode(ctx, dbgen.UpsertEpisodeParams{SeasonID: season.ID, EpisodeNumber: int32(e.EpisodeNumber), Name: e.Name, Overview: e.Overview, AirDate: dateValue(e.AirDate), RuntimeMinutes: int32(e.RuntimeMinutes), StillUrl: e.StillURL, ExternalID: e.ExternalID})
		if err != nil {
			return nil, err
		}
		out = append(out, convertEpisode(row, seasonID))
	}
	return out, nil
}
func (r *Repository) Episodes(ctx context.Context, m media.Media, number int) ([]media.Episode, error) {
	mid, err := r.MediaUUID(ctx, m.Provider, m.ExternalID)
	if err != nil {
		return nil, err
	}
	season, err := r.Q.GetSeasonByMediaAndNumber(ctx, dbgen.GetSeasonByMediaAndNumberParams{MediaID: mid, SeasonNumber: int32(number)})
	if err != nil {
		return nil, err
	}
	rows, err := r.Q.GetEpisodesBySeasonID(ctx, season.ID)
	if err != nil {
		return nil, err
	}
	seasonID := m.ID + ":season:" + fmt.Sprint(number)
	out := make([]media.Episode, 0, len(rows))
	for _, row := range rows {
		out = append(out, convertEpisode(row, seasonID))
	}
	return out, nil
}
func (r *Repository) Source(ctx context.Context, id string) (media.Source, error) {
	uid, e := UUID(id)
	if e != nil {
		return media.Source{}, pgx.ErrNoRows
	}
	s, e := r.Q.GetSourceByID(ctx, uid)
	if e != nil {
		return media.Source{}, e
	}
	p, e := r.Q.GetProviderByID(ctx, s.ProviderID)
	if e != nil {
		return media.Source{}, e
	}
	if s.ProviderID != p.ID {
		return media.Source{}, errors.New("unsupported source provider")
	}
	return convertSource(s, p.Name), nil
}
func (r *Repository) AddLibrary(ctx context.Context, m media.Media) error {
	mid, e := r.MediaUUID(ctx, m.Provider, m.ExternalID)
	if e != nil {
		return e
	}
	return r.Q.AddLibraryItem(ctx, mid)
}
func (r *Repository) RemoveLibrary(ctx context.Context, m media.Media) error {
	mid, e := r.MediaUUID(ctx, m.Provider, m.ExternalID)
	if e != nil {
		return e
	}
	return r.Q.RemoveLibraryItem(ctx, mid)
}
func (r *Repository) Library(ctx context.Context, page, limit int) ([]media.Media, error) {
	rows, e := r.Q.ListLibraryItems(ctx, dbgen.ListLibraryItemsParams{Limit: int32(limit), Offset: int32((page - 1) * limit)})
	if e != nil {
		return nil, e
	}
	out := make([]media.Media, 0, len(rows))
	for _, row := range rows {
		p, e := r.Q.GetProviderByID(ctx, row.ProviderID)
		if e != nil {
			return nil, e
		}
		out = append(out, convertMedia(row, p.Name))
	}
	return out, nil
}
