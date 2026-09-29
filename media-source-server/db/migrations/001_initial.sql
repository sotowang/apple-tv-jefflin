CREATE TABLE IF NOT EXISTS providers (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name TEXT NOT NULL UNIQUE, type TEXT NOT NULL,
 enabled BOOLEAN NOT NULL DEFAULT true, priority INTEGER NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS media (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), type TEXT NOT NULL, title TEXT NOT NULL,
 original_title TEXT NOT NULL DEFAULT '', year INTEGER, overview TEXT NOT NULL DEFAULT '',
 poster_url TEXT NOT NULL DEFAULT '', backdrop_url TEXT NOT NULL DEFAULT '',
 provider_id UUID NOT NULL REFERENCES providers(id), external_id TEXT NOT NULL,
 license_url TEXT NOT NULL DEFAULT '', rights TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(provider_id, external_id)
);
CREATE TABLE IF NOT EXISTS sources (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
 provider_id UUID NOT NULL REFERENCES providers(id), external_id TEXT NOT NULL,
 file_name TEXT NOT NULL, original_url TEXT NOT NULL DEFAULT '', quality TEXT NOT NULL DEFAULT '',
 container TEXT NOT NULL DEFAULT '', video_codec TEXT NOT NULL DEFAULT '', audio_codec TEXT NOT NULL DEFAULT '',
 bitrate BIGINT, direct_play BOOLEAN NOT NULL DEFAULT false, requires_proxy BOOLEAN NOT NULL DEFAULT false,
 status TEXT NOT NULL DEFAULT 'available', last_checked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(provider_id, external_id, file_name)
);
CREATE TABLE IF NOT EXISTS library_items (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), media_id UUID NOT NULL UNIQUE REFERENCES media(id) ON DELETE CASCADE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS source_health (
 source_id UUID PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
 status TEXT NOT NULL DEFAULT 'unknown', http_status INTEGER, latency_ms INTEGER,
 last_success_at TIMESTAMPTZ, last_failure_at TIMESTAMPTZ, checked_at TIMESTAMPTZ
);
INSERT INTO providers(name,type,priority) VALUES('archive','vod',100) ON CONFLICT(name) DO NOTHING;
