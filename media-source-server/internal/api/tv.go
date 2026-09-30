package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider"
)

func (a *API) getSeasons(ctx context.Context, m media.Media) ([]media.Season, error) {
	if m.Type != string(media.MediaTypeTV) {
		return []media.Season{}, nil
	}
	if found, ok := a.SeasonsCache.Get(m.ID); ok {
		return found, nil
	}
	p, ok := a.Registry.MetadataProviders[m.Provider]
	if !ok {
		return nil, provider.ErrNotFound
	}
	fresh, err := p.GetSeasons(ctx, m.ExternalID)
	if err != nil {
		return nil, err
	}
	if len(fresh) > 0 {
		fresh, err = a.Repo.UpsertSeasons(ctx, m, fresh)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errDatabase, err)
		}
	}
	if fresh == nil {
		fresh = []media.Season{}
	}
	a.SeasonsCache.Set(m.ID, fresh, a.Config.MediaTTL)
	return fresh, nil
}
func (a *API) getEpisodes(ctx context.Context, m media.Media, number int) ([]media.Episode, error) {
	if m.Type != string(media.MediaTypeTV) {
		return []media.Episode{}, nil
	}
	key := fmt.Sprintf("%s:%d", m.ID, number)
	if found, ok := a.EpisodesCache.Get(key); ok {
		return found, nil
	}
	seasons, err := a.getSeasons(ctx, m)
	if err != nil {
		return nil, err
	}
	exists := false
	for _, s := range seasons {
		if s.SeasonNumber == number {
			exists = true
			break
		}
	}
	if !exists {
		return nil, provider.ErrNotFound
	}
	p := a.Registry.MetadataProviders[m.Provider]
	fresh, err := p.GetEpisodes(ctx, m.ExternalID, number)
	if err != nil {
		return nil, err
	}
	if len(fresh) > 0 {
		fresh, err = a.Repo.UpsertEpisodes(ctx, m, number, fresh)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errDatabase, err)
		}
	}
	if fresh == nil {
		fresh = []media.Episode{}
	}
	a.EpisodesCache.Set(key, fresh, a.Config.MediaTTL)
	return fresh, nil
}
func (a *API) seasons(c *gin.Context) {
	m, err := a.getMedia(c.Request.Context(), c.Param("id"))
	if err != nil {
		a.providerFailure(c, err)
		return
	}
	items, err := a.getSeasons(c.Request.Context(), m)
	if err != nil {
		a.providerFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": toSeasonRecords(items)})
}
func seasonNumber(c *gin.Context) (int, bool) {
	n, err := strconv.Atoi(c.Param("seasonNumber"))
	if err != nil || n < 0 || n > 9999 {
		failure(c, 400, "INVALID_QUERY", "invalid season number")
		return 0, false
	}
	return n, true
}
func (a *API) episodes(c *gin.Context) {
	n, ok := seasonNumber(c)
	if !ok {
		return
	}
	m, err := a.getMedia(c.Request.Context(), c.Param("id"))
	if err != nil {
		a.providerFailure(c, err)
		return
	}
	items, err := a.getEpisodes(c.Request.Context(), m, n)
	if err != nil {
		a.providerFailure(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": toEpisodeRecords(items)})
}
func (a *API) episodeSources(c *gin.Context) {
	n, ok := seasonNumber(c)
	if !ok {
		return
	}
	episodeNumber, err := strconv.Atoi(c.Param("episodeNumber"))
	if err != nil || episodeNumber < 1 {
		failure(c, 400, "INVALID_QUERY", "invalid episode number")
		return
	}
	m, err := a.getMedia(c.Request.Context(), c.Param("id"))
	if err != nil {
		a.providerFailure(c, err)
		return
	}
	episodes, err := a.getEpisodes(c.Request.Context(), m, n)
	if err != nil {
		a.providerFailure(c, err)
		return
	}
	for _, e := range episodes {
		if e.EpisodeNumber == episodeNumber {
			e.SeasonNumber = n
			key := fmt.Sprintf("%s:season:%d:episode:%d", m.ID, n, episodeNumber)
			if cached, ok := a.SourcesCache.Get(key); ok {
				c.JSON(http.StatusOK, gin.H{"items": cached, "sources": cached})
				return
			}
			items, err := a.SourceResolver.ResolveEpisode(c.Request.Context(), m, e)
			if err != nil {
				a.providerFailure(c, err)
				return
			}
			items, ttl := a.prepareSources(items)
			a.SourcesCache.Set(key, items, ttl)
			c.JSON(http.StatusOK, gin.H{"items": items, "sources": items})
			return
		}
	}
	failure(c, 404, "EPISODE_NOT_FOUND", "episode not found")
}
