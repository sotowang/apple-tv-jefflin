# Media API

See the root README for deployment and routes. Go 1.25, Gin, pgx, sqlc, Resty, PostgreSQL, and an in-process TTL cache. The API runs the idempotent `db/migrations/001_initial.sql` at startup. `cmd/migrate` runs the same file explicitly. Never store video bodies or expiring upstream URLs in PostgreSQL.

Environment: `APP_ENV`, `HTTP_PORT`, `PUBLIC_BASE_URL`, `DATABASE_URL`, `REQUEST_TIMEOUT`, `API_KEY`, `PLAY_URL_SIGNING_SECRET`, `SEARCH_CACHE_TTL`, `MEDIA_CACHE_TTL`, `SOURCE_CACHE_TTL`, `LOG_LEVEL`. Production requires HTTPS `PUBLIC_BASE_URL` and secrets at least 32 characters.
