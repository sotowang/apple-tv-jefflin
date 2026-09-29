package tmdb

import (
	"context"
	"net/url"
	"strconv"

	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

func tvID(externalID string) (string, error) {
	kind, id, err := parseExternalID(externalID)
	if err != nil || kind != "tv" {
		return "", provider.ErrNotFound
	}
	return id, nil
}
func (c *Client) GetSeasons(ctx context.Context, externalID string) ([]media.Season, error) {
	id, err := tvID(externalID)
	if err != nil {
		return nil, err
	}
	var dto result
	if err = c.get(ctx, "/tv/"+url.PathEscape(id), nil, &dto); err != nil {
		return nil, err
	}
	if dto.ID == 0 {
		return nil, provider.ErrNotFound
	}
	out := make([]media.Season, 0, len(dto.Seasons))
	for _, s := range dto.Seasons {
		out = append(out, mapSeason("tmdb:tv:"+id, s))
	}
	return out, nil
}
func (c *Client) GetEpisodes(ctx context.Context, externalID string, seasonNumber int) ([]media.Episode, error) {
	id, err := tvID(externalID)
	if err != nil || seasonNumber < 0 || seasonNumber > 9999 {
		return nil, provider.ErrNotFound
	}
	var dto seasonDTO
	if err = c.get(ctx, "/tv/"+url.PathEscape(id)+"/season/"+strconv.Itoa(seasonNumber), nil, &dto); err != nil {
		return nil, err
	}
	seasonID := "tmdb:tv:" + id + ":season:" + strconv.Itoa(seasonNumber)
	out := make([]media.Episode, 0, len(dto.Episodes))
	for _, e := range dto.Episodes {
		out = append(out, mapEpisode(seasonID, e))
	}
	return out, nil
}
