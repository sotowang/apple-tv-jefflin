# Online Media V2 Phase 2

A personal, zero video storage media catalog for Jellyfin and Swiftfin. TMDB supplies movie and TV metadata; Internet Archive continues to supply its existing movie metadata, playable sources, Featured browse, and the Swiftfin Movies catalog. PostgreSQL stores selected metadata, source references, and library choices. The Go service issues ten minute signed play links for the Jellyfin Channel. A catalog sync writes `.strm` links and `.nfo` metadata to a standard Jellyfin Movies library for Swiftfin. The catalog sync does not download video; Jellyfin may fetch video when probing or transcoding.

## V2 Architecture

The Go API uses separate metadata and source providers. TMDB supplies localized Movie and TV metadata, including seasons and episodes. TMDB does not supply video streams. SourceResolver aggregates enabled source providers. Internet Archive remains a metadata and movie source provider; an optional CustomSourceProvider calls a user-controlled lookup API.

Search is transient: results do not create database rows. Detail requests upsert metadata into PostgreSQL. The API uses its existing in-memory TTL cache for search, detail, seasons, episodes, and sources; no Redis service is needed. Search ranks exact localized titles before original-title matches, prefixes, and fuzzy matches. TMDB popularity only breaks ties within a rank. Stable external IDs use `tmdb:movie:<id>`, `tmdb:tv:<id>`, and `archive:<identifier>`; database UUIDs stay internal.

Migration `002_media_v2.sql` adds release date, language, TMDB ID, and TV counts to `media`, plus `seasons` and `episodes` tables. It leaves `001_initial.sql`, existing Archive sources, and library data intact. The API applies this migration at startup. Existing Jellyfin Channel, signed play, catalog sync, and Swiftfin Movies library continue to use Archive sources; TMDB TV entries do not become playable in Swiftfin during this phase.

## Layout

- `media-source-server/`: Go API, sqlc queries, migrations, TMDB metadata provider, Archive provider, tests.
- `jellyfin-online-media-plugin/`: Jellyfin 12.1 Channel plugin, shared Media API client, and configuration page.
- `deploy/`: Docker Compose deployment, Caddy HTTPS routing, and a shared metadata-only movie catalog volume.

## DNS and VPS

Create A records for `jellyfin.example.com` and `media.example.com` pointing to the VPS public IPv4 address. Replace the example names in `.env` with your own domains. AAAA records are optional if IPv6 is configured. If using Cloudflare, start with **DNS only** to verify playback and WebSocket behavior.

Install Docker Engine and the Compose plugin on a Linux VPS. Allow inbound TCP 22, 80, 443 and UDP 443 (HTTP/3, optional). Do not publish PostgreSQL 5432, Jellyfin 8096, or API 8080. Only Caddy exposes public ports.

## Configure and deploy

```sh
cp deploy/.env.example deploy/.env
openssl rand -hex 32   # POSTGRES_PASSWORD
openssl rand -hex 32   # API_KEY
openssl rand -hex 32   # PLAY_URL_SIGNING_SECRET
```

Edit `deploy/.env`: set both hostnames, `PUBLIC_MEDIA_BASE_URL=https://<media host>`, and the three distinct generated secrets. Keep the file private. The values must be set before Compose will start. The media API also requires HTTPS for its public URL. For a normal deployment:

To enable TMDB metadata search, set `TMDB_API_TOKEN` to a TMDB API Read Access Token. `TMDB_LANGUAGE` defaults to `zh-CN`; `TMDB_REGION` defaults to `CN`. An empty token disables TMDB without stopping the API or Archive. The token is sent only to TMDB as a Bearer credential.

```env
TMDB_API_TOKEN=<TMDB API Read Access Token>
TMDB_LANGUAGE=zh-CN
TMDB_REGION=CN
```

```sh
./jellyfin-online-media-plugin/build.sh
cd deploy
./deploy.sh
```

`deploy.sh` builds, starts, shows container status, waits for PostgreSQL, API, and Jellyfin health, checks `https://<media host>/health`, then syncs the movie catalog. It needs working DNS and ports 80/443 for Caddy certificate issuance. Verify manually:

```sh
curl https://media.example.com/health
# {"status":"ok"}
docker compose ps
```

The API applies pending numbered migrations at startup before serving requests. `001_initial.sql` creates the V1 tables and Archive provider. For an explicit migration run, use `docker compose run --rm media-api /app/migrate`. Schema changes in later versions must use new numbered migrations; do not edit a migration already applied to production.

The deployment also syncs up to `CATALOG_LIMIT` (default 20) manually selected classic Archive films into the `catalog-data` volume. The sync checks each item's rights metadata for a recognizable public-domain or Creative Commons marker and requires a direct-play source candidate. This metadata is a hint, not a legal determination. Each film has only a direct Archive `.strm` URL and a small `.nfo` file. To refresh the catalog later, run `cd deploy && docker compose --profile catalog run --rm catalog-sync`, then scan the Jellyfin library.

## Jellyfin and Apple TV

1. Open `https://jellyfin.example.com`, create the Jellyfin administrator, and finish setup.
2. Build the plugin on the VPS or locally with .NET SDK 10 or Docker:

   ```sh
   ./jellyfin-online-media-plugin/build.sh
   ```

   This publishes the plugin and copies its runtime files into `deploy/plugins/OnlineMedia/`. If built locally, copy the full plugin directory to the same path on the VPS. This path is mounted at `/config/plugins/OnlineMedia` in Jellyfin.
3. Restart Jellyfin: `cd deploy && docker compose restart jellyfin`. In Dashboard → Plugins, confirm Online Media loaded. Open its configuration, enter `https://media.example.com`, the same `API_KEY` from `deploy/.env`, and a request timeout, then Save.
4. In Jellyfin Web, open Channels → 在线影视 / Online Media → **Featured**, **Library**, or **Search Examples**. Featured browses Archive's open source movies collection. Library displays only items explicitly added through the Library API; it does not bulk import remote media into Jellyfin. Search Examples are labeled preset queries, not a text search box. Select a movie to load a direct-play candidate and play.
5. In Jellyfin Dashboard → Libraries, add a **Movies** library named **在线影视** with folder `/media/online`. Disable remote metadata providers for this library so Jellyfin uses the supplied `movie.nfo` titles, then scan the library. On Apple TV or iPhone, open Swiftfin → Media → **在线影视** (the Movies library), and select a film. The older Channel entry has the same name but still shows “No Items” in Swiftfin; select the Movies library instead.

**Swiftfin compatibility:** The current Swiftfin library screen does not browse Jellyfin Channel items. It queries the generic `/Items` endpoint and filters a `Channel` parent to live TV programs, while this plugin supplies channel folders and movies through `/Channels/{channelId}/Items`. The Channel can therefore appear in Swiftfin with “No Items” even when Jellyfin Web shows Featured and its films. The separate Movies library exposes standard movie items through `/Items` and is the Swiftfin entry point.

The `.strm` entries point straight to stable `https://archive.org/download/...` URLs. Playback compatibility still depends on the chosen film's actual codecs and Swiftfin's player. Jellyfin may proxy or transcode if direct play fails, which can use VPS bandwidth; check the playback status before assuming zero video traffic.

For the Jellyfin Web Channel, the plugin calls the API for details and sources, selects the first direct-play candidate, requests a signed URL, and gives that URL to Jellyfin as a remote source. The separate Movies library uses its `.strm` URLs instead. The `directPlay` field is a **container-level candidate flag** for MP4/M4V/MOV; it is not a codec probe or a guarantee of H.264/AAC compatibility. Explicit incompatible codec metadata excludes a candidate. The plugin does not probe or transcode video.

## API

All `/api/v1` calls require `X-API-Key`; `/health` and signed `/play` do not. Errors use `{ "error": { "code": "...", "message": "..." } }`.

| Method | Route | Purpose |
|---|---|---|
| GET | `/health` | Service status |
| GET | `/api/v1/providers` | Enabled providers |
| GET | `/api/v1/search?q=...&page=1&limit=20` | TMDB and Archive metadata search, transient results; optional `provider=archive` or `provider=tmdb` |
| GET | `/api/v1/featured?page=1&limit=20` | Archive collection browse, independent of title search |
| GET | `/api/v1/media/:id` | Detail and media upsert |
| GET | `/api/v1/media/:id/sources` | Sources and source upsert |
| GET | `/api/v1/media/:id/seasons` | TV seasons |
| GET | `/api/v1/media/:id/seasons/:seasonNumber/episodes` | TV episodes |
| GET | `/api/v1/media/:id/seasons/:seasonNumber/episodes/:episodeNumber/sources` | Episode sources; empty until a source provider supports them |
| GET | `/api/v1/library` | Saved items |
| POST | `/api/v1/library/:mediaId` | Save item |
| DELETE | `/api/v1/library/:mediaId` | Remove item |
| POST | `/api/v1/sources/:id/play-url` | Ten minute HMAC signed link |
| GET | `/play/:sourceId?expires=...&token=...` | Validate then redirect 302 |
| HEAD | `/play/:sourceId?expires=...&token=...` | Validate then redirect 302 for HEAD probes |

Search IDs use `archive:<identifier>` as external API references; PostgreSQL media and source primary keys are UUIDs. Search and Featured do not fill PostgreSQL. Detail requests save media; source requests save stable file references. Transient CDN URLs and video bytes are never stored. Detail returns `rightsStatus`: `verified` for recognizable public domain/Creative Commons metadata, `restricted` for explicit restriction, or `unknown`. This is a metadata hint, not a legal determination. Public Archive access and collection membership alone do not establish reuse rights.

V2 search responses contain `items`, `page`, and `limit`. Sources responses contain both `sources` (for existing clients) and `items` (for V2 clients). Existing Archive response fields and routes remain available. TV detail exposes `type`, `releaseDate`, `originalLanguage`, `tmdbId`, `seasonCount`, and `episodeCount` where present. TMDB poster, backdrop, and still paths are returned as complete HTTPS URLs.

```sh
curl -G -H "X-API-Key: $API_KEY" --data-urlencode 'q=凡人修仙传' 'https://media.example.com/api/v1/search'
curl -H "X-API-Key: $API_KEY" 'https://media.example.com/api/v1/media/tmdb:tv:XXX'
curl -H "X-API-Key: $API_KEY" 'https://media.example.com/api/v1/media/tmdb:tv:XXX/seasons'
curl -H "X-API-Key: $API_KEY" 'https://media.example.com/api/v1/media/tmdb:tv:XXX/seasons/1/episodes'
curl -H "X-API-Key: $API_KEY" 'https://media.example.com/api/v1/media/tmdb:tv:XXX/sources'
curl -G -H "X-API-Key: $API_KEY" --data-urlencode 'q=night of the living dead' 'https://media.example.com/api/v1/search?provider=archive'
```

Replace `XXX` with the numeric ID returned by search. The same routes work against `http://localhost:8080` during local development, provided the required API configuration and PostgreSQL are running.

In non-production environments (`APP_ENV != production`), authenticated `GET /api/v1/debug/media/:id` returns media, sources, selected candidate, and a `resolved` object containing host, container, directPlay, and proxyRequired, without the full URL. The route is absent in production. The Go API does not serve video Range data: after the signed `/play` redirect, the client communicates directly with Archive/CDN, which handles Range requests. No video bytes pass through this VPS.

Example:

```sh
curl -H "X-API-Key: $API_KEY" "https://media.example.com/api/v1/search?q=night%20of%20the%20living%20dead"
```

## Database backup and restore

```sh
cd deploy
docker compose exec -T postgres pg_dump -U media media > media-backup.sql
# Restore only to a stopped or empty target database after making a separate backup:
cat media-backup.sql | docker compose exec -T postgres psql -U media -d media
```

Back up `jellyfin-config` as well if you want Jellyfin users and settings. No media files exist in these volumes.

## Development and checks

```sh
cd media-source-server
go test ./...
go vet ./...
go build ./...
# Optional PostgreSQL integration tests (point to a disposable database):
TEST_DATABASE_URL='postgres://media:...@localhost:5432/media?sslmode=disable' go test ./...
# Regenerate checked-in pgx queries after changing SQL:
docker run --rm -v "$PWD:/src" -w /src sqlc/sqlc:1.30.0 generate
cd ../jellyfin-online-media-plugin
dotnet build -c Release
cd ../deploy
docker compose config
```

Provider tests use `httptest.Server` and require no internet. Repository and end-to-end API tests run with `TEST_DATABASE_URL` and skip otherwise. Do not point integration tests at production; they create tables and write/delete test records.

## Limits and next steps

V2 Phase 1 adds TMDB metadata and TV browsing to the Go API. Swiftfin does not display the Channel's films; use the Movies library. Jellyfin's global text search does **not** search remote metadata beyond films already synced into the Movies library. The Channel has Featured, Library, and clearly labeled Search Examples; arbitrary text search exists only at the Go `/api/v1/search` endpoint. An MP4 extension does not guarantee direct play. There is no application-level source fallback, IPTV, health worker, subtitles, video proxy, or high availability. Actual Apple TV playback must be verified on your device and network.

Phase 3 could add Jellyfin TV catalog integration, provider-specific health telemetry, and a text search UI. This phase does not add scraping, downloads, proxies, torrents, DRM bypass, Redis, or Elasticsearch.

## V2 Phase 2 Source Resolution

TMDB is the metadata layer. `SourceResolver` calls enabled source providers in `SOURCE_PROVIDER_ORDER`, validates title/year and episode numbers, isolates upstream failures, removes duplicate URLs, and ranks direct play, proxy needs, quality, language, then provider order. `CustomSourceProvider` performs lookup against a fixed user-controlled API. It never crawls websites. The `items` response and legacy `sources` response contain the same unified sources. `/api/v1/providers` reports metadata and source registrations without active health checks.

Set `CUSTOM_SOURCE_ENABLED=true`, `CUSTOM_SOURCE_BASE_URL=https://your-source-api.example`, and optionally `CUSTOM_SOURCE_API_KEY`. Defaults: `CUSTOM_SOURCE_TIMEOUT=8s`, `CUSTOM_SOURCE_ALLOW_PRIVATE_NETWORK=false`, `SOURCE_PROVIDER_ORDER=custom,archive`, `SOURCE_CACHE_TTL=10m`. The private network flag is intended only for a trusted local test service. The fixed base URL must use HTTPS when the flag is false. Requests and redirects cannot switch to another host; DNS targets are checked before each connection.

Custom API contract:

```http
GET /v1/search/movie?title=流浪地球&year=2019
GET /v1/search/episode?title=凡人修仙传&season=1&episode=1
Authorization: Bearer <CUSTOM_SOURCE_API_KEY>
Accept: application/json
```

Return HTTP 200 with `{"items":[{"providerItemId":"ep-001","title":"凡人修仙传","season":1,"episode":1,"url":"https://example.com/episode1.m3u8","quality":"1080p","container":"hls","language":"zh-CN","directPlay":true}]}`. Movie items use `year` instead of `season` and `episode`. Optional fields: `externalId`, `originalTitle`, `requiresProxy`, `width`, `height`, `bitrate`, `expiresAt` (RFC3339), and `ephemeral`. HTTP 404 means no result. Only HTTPS playback URLs are accepted. Source URLs are held in memory under opaque signed IDs and are never persisted to PostgreSQL. Cache lifetime is at most 10 minutes and at most 30 seconds before `expiresAt`. Restarting the API invalidates these IDs.

Test the flow after configuring TMDB and a custom lookup API:

```sh
curl -G -H "X-API-Key: $API_KEY" --data-urlencode 'q=凡人修仙传' 'http://localhost:8080/api/v1/search?provider=tmdb'
curl -H "X-API-Key: $API_KEY" 'http://localhost:8080/api/v1/media/tmdb:tv:<id>/seasons/1/episodes'
curl -H "X-API-Key: $API_KEY" 'http://localhost:8080/api/v1/media/tmdb:tv:<id>/seasons/1/episodes/1/sources'
curl -X POST -H "X-API-Key: $API_KEY" 'http://localhost:8080/api/v1/sources/<source-id>/play-url'
curl -I '<returned-play-url>'
```

The last request returns a 302 to a direct HTTPS stream; Go does not relay video bytes. For signed upstream URLs, clients should request a fresh source and play URL near playback time.

## Release and plugin compatibility

- Tested Jellyfin version: `Jellyfin.Server 12.1.0.0` from the official 12.1 image; deployment pins `jellyfin/jellyfin:12.1.20260915-010956` to keep the server version stable.
- Plugin target: `Jellyfin.Controller` 12.1.0 and `Jellyfin.Model` 12.1.0.
- .NET: `net10.0`; build with .NET SDK 10. The official 12.1 image contains `curl` for its `/health` check.
- `IPluginServiceRegistrator` registers one reusable `MediaApiClient` and `IChannel → OnlineChannel`; Jellyfin constructs the Channel with DI. The plugin folder is `deploy/plugins/OnlineMedia/`, mounted at `/config/plugins/OnlineMedia/`. `build.sh` publishes and copies plugin runtime files there. Restart Jellyfin after replacing the DLL.

## Smoke test

Install `jq` and `curl`, then run after the API is reachable and Featured contains at least one playable item:

```sh
MEDIA_API_URL=https://media.example.com API_KEY='<same key as deploy/.env>' ./deploy/smoke-test.sh
```

The script checks `/health`, Featured, media detail, sources, signed `/play`, and the 302 Location host. It uses HEAD for `/play`, does not follow the redirect, and never downloads video. To include it in deployment checks, set `RUN_SMOKE_TEST=true` before `./deploy/deploy.sh`; the default is off. The deployment script also checks PostgreSQL, Media API, Jellyfin, Caddy, and the public HTTPS health endpoint. Do not run the deployment script over a live production stack unless you intend to update it.

The migration runner records filenames in `schema_migrations` and applies pending numbered SQL files in order under a PostgreSQL advisory lock. Repeated startup skips migrations already recorded. Add `002_xxx.sql`, `003_xxx.sql`, and so on for future schema changes. Never edit `001_initial.sql` after it has reached production.

## Apple TV Test Checklist

1. Open Swiftfin on Apple TV and log in to Jellyfin.
2. Open Media → 在线影视 (the Movies library) and select a film.
3. Select a film and press Play.
4. On the server, run `cd deploy && docker compose logs -f jellyfin media-api`.
5. Confirm the film starts from an Archive URL and check Jellyfin's playback status for direct play. The plugin's `Online Media item` and signed `/play` logs apply to the separate Jellyfin Web Channel, not this Movies library.
6. Confirm the film starts and the Apple TV player reports direct play where available.

Run `docker stats` during playback and watch the `jellyfin` NET I/O counters and overall VPS outbound traffic. Sustained MB/s growth means investigate whether Jellyfin is transcoding or proxying instead of Swiftfin fetching the Archive file directly.

## Troubleshooting order

| Symptom | Check |
|---|---|
| Plugin absent | Jellyfin logs, `/config/plugins/OnlineMedia`, `OnlineMedia.dll`, `net10.0` build, Jellyfin 12.1 SDK match |
| Channel opens but is empty | Media API URL and key in plugin settings, `/api/v1/featured`, container DNS and `/health` |
| Channel works in Jellyfin Web but shows “No Items” in Swiftfin | Open the separate Movies library under Media; Swiftfin does not browse the Channel's folders |
| Movies library is empty | Check `docker compose --profile catalog run --rm catalog-sync`, the library path `/media/online`, and rescan the Movies library |
| Selecting a film fails | `/api/v1/media/:id`, `/sources`, candidate count, `directPlay` and `requiresProxy` |
| Movies library playback fails | Archive URL in `movie.strm`, HTTP redirect, Jellyfin playback status, Swiftfin codec support |
| Channel playback fails in Jellyfin Web | `/play-url`, signed `/play` 302 with `curl -I`, Archive source availability |
| VPS traffic is high | Jellyfin playback status for transcoding/proxying and whether Swiftfin is using direct play |

Go API supports arbitrary text search at `/api/v1/search`. Swiftfin/Jellyfin Channel V1 does not yet provide arbitrary text input; its UI is Featured, Library, and Search Examples.
