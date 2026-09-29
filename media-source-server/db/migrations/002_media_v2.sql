ALTER TABLE media ADD COLUMN IF NOT EXISTS release_date DATE;
ALTER TABLE media ADD COLUMN IF NOT EXISTS original_language TEXT NOT NULL DEFAULT '';
ALTER TABLE media ADD COLUMN IF NOT EXISTS tmdb_id INTEGER;
ALTER TABLE media ADD COLUMN IF NOT EXISTS season_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media ADD COLUMN IF NOT EXISTS episode_count INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS seasons (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), media_id UUID NOT NULL REFERENCES media(id) ON DELETE CASCADE,
 season_number INTEGER NOT NULL CHECK (season_number >= 0), name TEXT NOT NULL DEFAULT '',
 overview TEXT NOT NULL DEFAULT '', poster_url TEXT NOT NULL DEFAULT '', air_date DATE,
 episode_count INTEGER NOT NULL DEFAULT 0, external_id TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(media_id, season_number)
);
CREATE TABLE IF NOT EXISTS episodes (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(), season_id UUID NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
 episode_number INTEGER NOT NULL CHECK (episode_number >= 0), name TEXT NOT NULL DEFAULT '',
 overview TEXT NOT NULL DEFAULT '', air_date DATE, runtime_minutes INTEGER NOT NULL DEFAULT 0,
 still_url TEXT NOT NULL DEFAULT '', external_id TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(season_id, episode_number)
);
INSERT INTO providers(name,type,priority) VALUES ('tmdb','metadata',200) ON CONFLICT(name) DO NOTHING;
