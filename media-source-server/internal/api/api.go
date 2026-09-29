package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"online-media/media-source-server/internal/cache"
	"online-media/media-source-server/internal/config"
	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/provider/archive"
	"online-media/media-source-server/internal/repository"
	"online-media/media-source-server/internal/security"
	"strconv"
	"strings"
	"time"
)

var errDatabase = errors.New("database error")

type API struct {
	Config       config.Config
	Repo         *repository.Repository
	Providers    map[string]media.Provider
	SearchCache  *cache.TTL[string, []media.Media]
	MediaCache   *cache.TTL[string, media.Media]
	SourcesCache *cache.TTL[string, []media.Source]
	ResolveCache *cache.TTL[string, media.ResolvedStream]
}

func New(cfg config.Config, repo *repository.Repository, providers ...media.Provider) *API {
	p := map[string]media.Provider{}
	for _, v := range providers {
		p[v.Name()] = v
	}
	return &API{cfg, repo, p, cache.New[string, []media.Media](time.Minute), cache.New[string, media.Media](time.Minute), cache.New[string, []media.Source](time.Minute), cache.New[string, media.ResolvedStream](time.Minute)}
}
func failure(c *gin.Context, status int, code, msg string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": msg}})
}
func logMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id := hex.EncodeToString(b)
		c.Header("X-Request-ID", id)
		requestContext, upstreamCode := media.WithUpstreamStatus(c.Request.Context())
		c.Request = c.Request.WithContext(requestContext)
		c.Next()
		provider, _ := c.Get("provider")
		mediaID, _ := c.Get("mediaId")
		sourceID, _ := c.Get("sourceId")
		upstreamStatus := *upstreamCode
		slog.Info("request", "requestId", id, "method", c.Request.Method, "endpoint", c.FullPath(), "provider", provider, "mediaId", mediaID, "sourceId", sourceID, "latencyMs", time.Since(start).Milliseconds(), "httpStatus", c.Writer.Status(), "upstreamStatus", upstreamStatus)
	}
}
func (a *API) auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !security.Equal(c.GetHeader("X-API-Key"), a.Config.APIKey) {
			failure(c, 401, "INVALID_API_KEY", "invalid API key")
			c.Abort()
			return
		}
		c.Next()
	}
}
func (a *API) Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), logMiddleware())
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	v := r.Group("/api/v1", a.auth())
	v.GET("/providers", a.providers)
	v.GET("/search", a.search)
	v.GET("/featured", a.featured)
	v.GET("/media/:id", a.detail)
	v.GET("/media/:id/sources", a.sources)
	if !strings.EqualFold(a.Config.AppEnv, "production") {
		v.GET("/debug/media/:id", a.debugMedia)
	}
	v.GET("/library", a.library)
	v.POST("/library/:mediaId", a.addLibrary)
	v.DELETE("/library/:mediaId", a.removeLibrary)
	v.POST("/sources/:id/play-url", a.playURL)
	r.GET("/play/:sourceId", a.play)
	r.HEAD("/play/:sourceId", a.play)
	return r
}
func parsePage(c *gin.Context) (int, int, bool) {
	page, e1 := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, e2 := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if e1 != nil || e2 != nil || page < 1 || page > 100 || limit < 1 || limit > 50 {
		failure(c, 400, "INVALID_QUERY", "invalid page or limit")
		return 0, 0, false
	}
	return page, limit, true
}
func (a *API) providers(c *gin.Context) {
	rows, e := a.Repo.Providers(c.Request.Context())
	if e != nil {
		failure(c, 500, "DATABASE_ERROR", "database error")
		return
	}
	out := []gin.H{}
	for _, p := range rows {
		out = append(out, gin.H{"name": p.Name, "type": p.Type, "priority": p.Priority})
	}
	c.JSON(200, gin.H{"providers": out})
}
func (a *API) search(c *gin.Context) {
	c.Set("provider", "archive")
	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 2 || len(q) > 120 {
		failure(c, 400, "INVALID_QUERY", "query must be 2-120 bytes")
		return
	}
	page, limit, ok := parsePage(c)
	if !ok {
		return
	}
	key := fmt.Sprintf("%s:%d:%d", q, page, limit)
	if found, ok := a.SearchCache.Get(key); ok {
		c.JSON(200, gin.H{"items": found})
		return
	}
	items := []media.Media{}
	for name, p := range a.Providers {
		if _, e := a.Repo.Provider(c.Request.Context(), name); e != nil {
			if errors.Is(e, pgx.ErrNoRows) {
				continue
			}
			failure(c, 500, "DATABASE_ERROR", "database error")
			return
		}
		found, e := p.Search(c.Request.Context(), media.SearchQuery{Query: q, Page: page, Limit: limit})
		if e != nil {
			a.providerFailure(c, e)
			return
		}
		items = append(items, found...)
	}
	a.SearchCache.Set(key, items, a.Config.SearchTTL)
	c.JSON(200, gin.H{"items": items})
}
func (a *API) featured(c *gin.Context) {
	c.Set("provider", "archive")
	page, limit, ok := parsePage(c)
	if !ok {
		return
	}
	key := fmt.Sprintf("featured:%d:%d", page, limit)
	if found, ok := a.SearchCache.Get(key); ok {
		c.JSON(200, gin.H{"items": found})
		return
	}
	items := []media.Media{}
	for name, provider := range a.Providers {
		browser, ok := provider.(media.BrowseProvider)
		if !ok {
			continue
		}
		if _, err := a.Repo.Provider(c.Request.Context(), name); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			failure(c, 500, "DATABASE_ERROR", "database error")
			return
		}
		found, err := browser.Featured(c.Request.Context(), page, limit)
		if err != nil {
			a.providerFailure(c, err)
			return
		}
		items = append(items, found...)
	}
	a.SearchCache.Set(key, items, a.Config.SearchTTL)
	c.JSON(200, gin.H{"items": items})
}
func splitID(id string) (string, string, bool) {
	s := strings.SplitN(id, ":", 2)
	if len(s) != 2 || s[0] == "" || s[1] == "" {
		return "", "", false
	}
	return s[0], s[1], true
}
func (a *API) getMedia(ctx context.Context, id string) (media.Media, error) {
	if m, ok := a.MediaCache.Get(id); ok {
		return m, nil
	}
	provider, external, ok := splitID(id)
	if !ok {
		return media.Media{}, archive.ErrNotFound
	}
	p, ok := a.Providers[provider]
	if !ok {
		return media.Media{}, archive.ErrNotFound
	}
	if _, e := a.Repo.Provider(ctx, provider); e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return media.Media{}, archive.ErrNotFound
		}
		return media.Media{}, fmt.Errorf("%w: %v", errDatabase, e)
	}
	m, e := a.Repo.MediaByExternal(ctx, provider, external)
	if e == nil {
		a.MediaCache.Set(id, m, a.Config.MediaTTL)
		return m, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return media.Media{}, fmt.Errorf("%w: %v", errDatabase, e)
	}
	fresh, e := p.GetMedia(ctx, external)
	if e != nil {
		return media.Media{}, e
	}
	m, e = a.Repo.UpsertMedia(ctx, *fresh)
	if e != nil {
		return media.Media{}, fmt.Errorf("%w: %v", errDatabase, e)
	}
	a.MediaCache.Set(id, m, a.Config.MediaTTL)
	return m, nil
}
func isTimeout(e error) bool {
	var n net.Error
	return errors.Is(e, context.DeadlineExceeded) || (errors.As(e, &n) && n.Timeout())
}
func (a *API) providerFailure(c *gin.Context, e error) {
	if errors.Is(e, errDatabase) {
		failure(c, 500, "DATABASE_ERROR", "database error")
	} else if errors.Is(e, archive.ErrNotFound) {
		failure(c, 404, "MEDIA_NOT_FOUND", "media not found")
	} else if errors.Is(e, archive.ErrNoPlayable) {
		failure(c, 404, "NO_PLAYABLE_SOURCE", "no playable source")
	} else if isTimeout(e) {
		failure(c, 504, "UPSTREAM_TIMEOUT", "upstream timeout")
	} else {
		failure(c, 502, "PROVIDER_ERROR", "provider error")
	}
}
func (a *API) detail(c *gin.Context) {
	c.Set("mediaId", c.Param("id"))
	if provider, _, ok := splitID(c.Param("id")); ok {
		c.Set("provider", provider)
	}
	m, e := a.getMedia(c.Request.Context(), c.Param("id"))
	if e != nil {
		a.providerFailure(c, e)
		return
	}
	c.JSON(200, m)
}
func (a *API) sources(c *gin.Context) {
	c.Set("mediaId", c.Param("id"))
	if provider, _, ok := splitID(c.Param("id")); ok {
		c.Set("provider", provider)
	}
	m, e := a.getMedia(c.Request.Context(), c.Param("id"))
	if e != nil {
		a.providerFailure(c, e)
		return
	}
	list, e := a.getSources(c.Request.Context(), m)
	if e != nil {
		a.providerFailure(c, e)
		return
	}
	c.JSON(200, gin.H{"sources": list})
}
func (a *API) getSources(ctx context.Context, m media.Media) ([]media.Source, error) {
	if cached, ok := a.SourcesCache.Get(m.ID); ok {
		return cached, nil
	}
	list, err := a.Repo.Sources(ctx, m.Provider, m.ExternalID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errDatabase, err)
	}
	if len(list) == 0 {
		list, err = a.Providers[m.Provider].GetSources(ctx, m)
		if err != nil {
			return nil, err
		}
		list, err = a.Repo.UpsertSources(ctx, m, list)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errDatabase, err)
		}
	}
	a.SourcesCache.Set(m.ID, list, a.Config.SourceTTL)
	return list, nil
}
func (a *API) debugMedia(c *gin.Context) {
	c.Set("mediaId", c.Param("id"))
	if provider, _, ok := splitID(c.Param("id")); ok {
		c.Set("provider", provider)
	}
	m, err := a.getMedia(c.Request.Context(), c.Param("id"))
	if err != nil {
		a.providerFailure(c, err)
		return
	}
	sources, err := a.getSources(c.Request.Context(), m)
	if err != nil {
		a.providerFailure(c, err)
		return
	}
	var selected *media.Source
	host := ""
	proxyRequired := false
	for i := range sources {
		if sources[i].DirectPlay && !sources[i].RequiresProxy {
			selected = &sources[i]
			break
		}
	}
	if selected != nil {
		resolved, resolveErr := a.Providers[selected.Provider].Resolve(c.Request.Context(), *selected)
		if resolveErr != nil {
			a.providerFailure(c, resolveErr)
			return
		}
		parsed, parseErr := url.Parse(resolved.URL)
		if parseErr == nil {
			host = parsed.Hostname()
		}
		proxyRequired = resolved.ProxyRequired
	}
	resolvedInfo := gin.H{"host": host, "container": "", "directPlay": false, "proxyRequired": false}
	if selected != nil {
		resolvedInfo["container"] = selected.Container
		resolvedInfo["directPlay"] = selected.DirectPlay
		resolvedInfo["proxyRequired"] = proxyRequired
		c.Set("sourceId", selected.ID)
	}
	c.JSON(200, gin.H{"media": m, "sources": sources, "selectedSource": selected, "resolved": resolvedInfo})
}
func (a *API) library(c *gin.Context) {
	page, limit, ok := parsePage(c)
	if !ok {
		return
	}
	items, e := a.Repo.Library(c.Request.Context(), page, limit)
	if e != nil {
		failure(c, 500, "DATABASE_ERROR", "database error")
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func (a *API) addLibrary(c *gin.Context) {
	m, e := a.getMedia(c.Request.Context(), c.Param("mediaId"))
	if e != nil {
		a.providerFailure(c, e)
		return
	}
	if e = a.Repo.AddLibrary(c.Request.Context(), m); e != nil {
		failure(c, 500, "DATABASE_ERROR", "database error")
		return
	}
	c.Status(204)
}
func (a *API) removeLibrary(c *gin.Context) {
	m, e := a.getMedia(c.Request.Context(), c.Param("mediaId"))
	if e != nil {
		a.providerFailure(c, e)
		return
	}
	if e = a.Repo.RemoveLibrary(c.Request.Context(), m); e != nil {
		failure(c, 500, "DATABASE_ERROR", "database error")
		return
	}
	c.Status(204)
}
func (a *API) playURL(c *gin.Context) {
	id := c.Param("id")
	c.Set("sourceId", id)
	source, e := a.Repo.Source(c.Request.Context(), id)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			failure(c, 404, "SOURCE_NOT_FOUND", "source not found")
		} else {
			failure(c, 500, "DATABASE_ERROR", "database error")
		}
		return
	}
	c.Set("provider", source.Provider)
	c.Set("mediaId", source.MediaID)
	expires := time.Now().Add(10 * time.Minute)
	v := url.Values{}
	v.Set("expires", strconv.FormatInt(expires.Unix(), 10))
	v.Set("token", security.Sign(a.Config.SigningSecret, id, expires.Unix()))
	slog.Info("play URL created", "sourceId", id)
	c.JSON(200, gin.H{"url": a.Config.PublicBaseURL + "/play/" + url.PathEscape(id) + "?" + v.Encode(), "expiresAt": expires.UTC().Format(time.RFC3339)})
}
func (a *API) play(c *gin.Context) {
	id := c.Param("sourceId")
	c.Set("sourceId", id)
	e := security.Verify(a.Config.SigningSecret, id, c.Query("expires"), c.Query("token"), time.Now())
	if e != nil {
		if errors.Is(e, security.ErrExpired) {
			failure(c, 401, "PLAY_TOKEN_EXPIRED", "play token expired")
		} else {
			failure(c, 401, "INVALID_PLAY_TOKEN", "invalid play token")
		}
		return
	}
	s, e := a.Repo.Source(c.Request.Context(), id)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			failure(c, 404, "SOURCE_NOT_FOUND", "source not found")
		} else {
			failure(c, 500, "DATABASE_ERROR", "database error")
		}
		return
	}
	c.Set("provider", s.Provider)
	c.Set("mediaId", s.MediaID)
	var resolved media.ResolvedStream
	if v, ok := a.ResolveCache.Get(id); ok {
		resolved = v
	} else {
		p, ok := a.Providers[s.Provider]
		if !ok {
			failure(c, 502, "PROVIDER_ERROR", "provider unavailable")
			return
		}
		v, e := p.Resolve(c.Request.Context(), s)
		if e != nil {
			a.providerFailure(c, e)
			return
		}
		resolved = *v
		a.ResolveCache.Set(id, resolved, 10*time.Minute)
	}
	if resolved.ProxyRequired || !strings.HasPrefix(resolved.URL, "https://") {
		failure(c, 502, "NO_PLAYABLE_SOURCE", "direct HTTPS source unavailable")
		return
	}
	parsed, parseErr := url.Parse(resolved.URL)
	if parseErr != nil || parsed.Hostname() == "" {
		failure(c, 502, "NO_PLAYABLE_SOURCE", "invalid direct source URL")
		return
	}
	slog.Info("play redirect", "sourceId", id, "provider", s.Provider, "targetHost", parsed.Hostname())
	c.Redirect(http.StatusFound, resolved.URL)
}
