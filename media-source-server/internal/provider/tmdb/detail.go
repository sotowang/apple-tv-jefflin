package tmdb

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

func parseExternalID(externalID string) (string, string, error) {
	parts := strings.Split(externalID, ":")
	if len(parts) != 2 || (parts[0] != "movie" && parts[0] != "tv") {
		return "", "", provider.ErrNotFound
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil || id <= 0 {
		return "", "", provider.ErrNotFound
	}
	return parts[0], strconv.Itoa(id), nil
}
func (c *Client) GetMedia(ctx context.Context, externalID string) (*media.Media, error) {
	kind, id, err := parseExternalID(externalID)
	if err != nil {
		return nil, err
	}
	var dto result
	if err = c.get(ctx, "/"+kind+"/"+url.PathEscape(id), nil, &dto); err != nil {
		return nil, err
	}
	if dto.ID == 0 {
		return nil, provider.ErrNotFound
	}
	m := mapMedia(dto, kind)
	return &m, nil
}
