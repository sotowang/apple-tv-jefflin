package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port, PublicBaseURL, DatabaseURL, APIKey, SigningSecret, LogLevel, AppEnv string
	TMDBAPIToken, TMDBLanguage, TMDBRegion                                    string
	CustomSourceBaseURL, CustomSourceAPIKey, SourceProviderOrder              string
	CustomSourceEnabled, CustomSourceAllowPrivateNetwork                      bool
	CustomSourceTimeout                                                       time.Duration
	RequestTimeout, SearchTTL, MediaTTL, SourceTTL                            time.Duration
}

func env(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}
func duration(k, def string) (time.Duration, error) { return time.ParseDuration(env(k, def)) }
func Load() (Config, error) {
	c := Config{Port: env("HTTP_PORT", "8080"), PublicBaseURL: strings.TrimRight(env("PUBLIC_BASE_URL", ""), "/"), DatabaseURL: os.Getenv("DATABASE_URL"), APIKey: os.Getenv("API_KEY"), SigningSecret: os.Getenv("PLAY_URL_SIGNING_SECRET"), LogLevel: env("LOG_LEVEL", "info"), AppEnv: env("APP_ENV", "production"), TMDBAPIToken: os.Getenv("TMDB_API_TOKEN"), TMDBLanguage: env("TMDB_LANGUAGE", "zh-CN"), TMDBRegion: env("TMDB_REGION", "CN"), CustomSourceBaseURL: os.Getenv("CUSTOM_SOURCE_BASE_URL"), CustomSourceAPIKey: os.Getenv("CUSTOM_SOURCE_API_KEY"), SourceProviderOrder: env("SOURCE_PROVIDER_ORDER", "custom,archive"), CustomSourceEnabled: strings.EqualFold(os.Getenv("CUSTOM_SOURCE_ENABLED"), "true"), CustomSourceAllowPrivateNetwork: strings.EqualFold(os.Getenv("CUSTOM_SOURCE_ALLOW_PRIVATE_NETWORK"), "true")}
	var e error
	c.RequestTimeout, e = duration("REQUEST_TIMEOUT", "10s")
	if e != nil {
		return c, e
	}
	c.SearchTTL, e = duration("SEARCH_CACHE_TTL", "10m")
	if e != nil {
		return c, e
	}
	c.MediaTTL, e = duration("MEDIA_CACHE_TTL", "1h")
	if e != nil {
		return c, e
	}
	c.SourceTTL, e = duration("SOURCE_CACHE_TTL", "10m")
	if e != nil {
		return c, e
	}
	c.CustomSourceTimeout, e = duration("CUSTOM_SOURCE_TIMEOUT", "8s")
	if e != nil {
		return c, e
	}
	if c.DatabaseURL == "" || len(c.APIKey) < 32 || len(c.SigningSecret) < 32 || !strings.HasPrefix(c.PublicBaseURL, "https://") {
		return c, errors.New("DATABASE_URL, HTTPS PUBLIC_BASE_URL, and strong API_KEY/PLAY_URL_SIGNING_SECRET required")
	}
	return c, nil
}
