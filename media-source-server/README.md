# Media API

See the root README for deployment and routes. Go 1.25, Gin, pgx, sqlc, Resty, PostgreSQL, and an in-process TTL cache. The API runs the idempotent `db/migrations/001_initial.sql` at startup. `cmd/migrate` runs the same file explicitly. Never store video bodies or expiring upstream URLs in PostgreSQL.

`/api/v1/featured` browses the Archive open source movies collection independently of text search. Detail computes conservative `rightsStatus` from rights/license metadata. `APP_ENV != production` enables authenticated `/api/v1/debug/media/:id`; production does not register that route. `/play` only validates and redirects, including HEAD requests. Upstream handles video Range requests.

Environment: `APP_ENV`, `HTTP_PORT`, `PUBLIC_BASE_URL`, `DATABASE_URL`, `REQUEST_TIMEOUT`, `API_KEY`, `PLAY_URL_SIGNING_SECRET`, `SEARCH_CACHE_TTL`, `MEDIA_CACHE_TTL`, `SOURCE_CACHE_TTL`, `LOG_LEVEL`. Production requires HTTPS `PUBLIC_BASE_URL` and secrets at least 32 characters.
