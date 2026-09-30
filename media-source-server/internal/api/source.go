package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"online-media/media-source-server/internal/media"
	"online-media/media-source-server/internal/security"
)

func (a *API) sourceTTL(s media.Source) time.Duration {
	ttl := a.Config.SourceTTL
	if ttl <= 0 || ttl > 10*time.Minute {
		ttl = 10 * time.Minute
	}
	if s.ExpiresAt != nil {
		until := time.Until(*s.ExpiresAt) - 30*time.Second
		if until < ttl {
			ttl = until
		}
	}
	return ttl
}
func (a *API) prepareSources(items []media.Source) ([]media.Source, time.Duration) {
	out := make([]media.Source, 0, len(items))
	ttl := a.Config.SourceTTL
	if ttl <= 0 || ttl > 10*time.Minute {
		ttl = 10 * time.Minute
	}
	for _, s := range items {
		sttl := a.sourceTTL(s)
		if sttl <= 0 {
			continue
		}
		if sttl < ttl {
			ttl = sttl
		}
		if s.Ephemeral || s.URL != "" {
			s.Ephemeral = true
			if s.ExpiresAt == nil {
				expires := time.Now().Add(sttl + 30*time.Second)
				s.ExpiresAt = &expires
			}
			if s.ID == "" {
				nonce := make([]byte, 16)
				if _, err := rand.Read(nonce); err != nil {
					continue
				}
				opaque := hex.EncodeToString(nonce)
				signature := security.Sign(a.Config.SigningSecret, "ephemeral:"+s.Provider+":"+opaque, 0)
				s.ID = fmt.Sprintf("ephemeral:%s:%s.%s", s.Provider, opaque, signature)
			}
			a.EphemeralSources.Set(s.ID, s, sttl)
		}
		out = append(out, s)
	}
	return out, ttl
}
func (a *API) ephemeralSource(id string) (media.Source, bool) {
	if !strings.HasPrefix(id, "ephemeral:") {
		return media.Source{}, false
	}
	parts := strings.Split(id, ":")
	if len(parts) != 3 {
		return media.Source{}, false
	}
	tail := strings.Split(parts[2], ".")
	if len(tail) != 2 || !security.Equal(tail[1], security.Sign(a.Config.SigningSecret, "ephemeral:"+parts[1]+":"+tail[0], 0)) {
		return media.Source{}, false
	}
	s, ok := a.EphemeralSources.Get(id)
	return s, ok
}
