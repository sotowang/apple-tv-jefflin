# Online Media V1

A personal, zero video storage Internet Archive browser for Jellyfin and Swiftfin. PostgreSQL stores metadata, source references, and library choices. The Go service issues ten minute signed play links and responds with HTTP 302 to the Archive file. The VPS does not download, proxy, cache, or transcode video.

## Layout

- `media-source-server/`: Go API, sqlc queries, SQL migration, Archive provider, tests.
- `jellyfin-online-media-plugin/`: Jellyfin 12.1 Channel plugin, shared Media API client, and configuration page.
- `deploy/`: four service Docker Compose deployment and Caddy HTTPS routing.

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

```sh
./jellyfin-online-media-plugin/build.sh
cd deploy
./deploy.sh
```

`deploy.sh` builds, starts, shows container status, waits for PostgreSQL, API, and Jellyfin health, then calls `https://<media host>/health`. It needs working DNS and ports 80/443 for Caddy certificate issuance. Verify manually:

```sh
curl https://media.example.com/health
# {"status":"ok"}
docker compose ps
```

The API applies pending numbered migrations at startup before serving requests. `001_initial.sql` creates the V1 tables and Archive provider. For an explicit migration run, use `docker compose run --rm media-api /app/migrate`. Schema changes in later versions must use new numbered migrations; do not edit a migration already applied to production.

## Jellyfin and Apple TV

1. Open `https://jellyfin.example.com`, create the Jellyfin administrator, and finish setup.
2. Build the plugin on the VPS or locally with .NET SDK 10 or Docker:

   ```sh
   ./jellyfin-online-media-plugin/build.sh
   ```

   This publishes the plugin and copies its runtime files into `deploy/plugins/OnlineMedia/`. If built locally, copy the full plugin directory to the same path on the VPS. This path is mounted at `/config/plugins/OnlineMedia` in Jellyfin.
3. Restart Jellyfin: `cd deploy && docker compose restart jellyfin`. In Dashboard → Plugins, confirm Online Media loaded. Open its configuration, enter `https://media.example.com`, the same `API_KEY` from `deploy/.env`, and a request timeout, then Save.
4. Install Swiftfin on Apple TV, connect to `https://jellyfin.example.com`, and sign in. Open Channels → 在线影视 / Online Media → **Featured**, **Library**, or **Search Examples**. Featured browses Archive's open source movies collection. Library displays only items explicitly added through the Library API; it does not bulk import remote media into Jellyfin. Search Examples are labeled preset queries, not a text search box. Select a movie to load a direct-play candidate and play.

The plugin calls the API for details and sources when Jellyfin requests media information, selects the first direct-play candidate, requests a signed URL, and gives that URL to Jellyfin as a remote source. Jellyfin should hand it to Swiftfin, which follows `/play` → 302 → Archive file. The `directPlay` field is a **container-level candidate flag** for MP4/M4V/MOV; it is not a codec probe or a guarantee of H.264/AAC compatibility. Explicit incompatible codec metadata excludes a candidate. The plugin does not probe or transcode video.

## API

All `/api/v1` calls require `X-API-Key`; `/health` and signed `/play` do not. Errors use `{ "error": { "code": "...", "message": "..." } }`.

| Method | Route | Purpose |
|---|---|---|
| GET | `/health` | Service status |
| GET | `/api/v1/providers` | Enabled providers |
| GET | `/api/v1/search?q=...&page=1&limit=20` | Archive movie title search, transient results |
| GET | `/api/v1/featured?page=1&limit=20` | Archive collection browse, independent of title search |
| GET | `/api/v1/media/:id` | Detail and media upsert |
| GET | `/api/v1/media/:id/sources` | Sources and source upsert |
| GET | `/api/v1/library` | Saved items |
| POST | `/api/v1/library/:mediaId` | Save item |
| DELETE | `/api/v1/library/:mediaId` | Remove item |
| POST | `/api/v1/sources/:id/play-url` | Ten minute HMAC signed link |
| GET | `/play/:sourceId?expires=...&token=...` | Validate then redirect 302 |
| HEAD | `/play/:sourceId?expires=...&token=...` | Validate then redirect 302 for HEAD probes |

Search IDs use `archive:<identifier>` as external API references; PostgreSQL media and source primary keys are UUIDs. Search and Featured do not fill PostgreSQL. Detail requests save media; source requests save stable file references. Transient CDN URLs and video bytes are never stored. Detail returns `rightsStatus`: `verified` for recognizable public domain/Creative Commons metadata, `restricted` for explicit restriction, or `unknown`. This is a metadata hint, not a legal determination. Public Archive access and collection membership alone do not establish reuse rights.

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

V1 supports Internet Archive, a Jellyfin Channel, Swiftfin browsing, PostgreSQL metadata, and 302 direct redirects. Jellyfin's global text search does **not** search Archive. The Channel has Featured, Library, and clearly labeled Search Examples; arbitrary text search exists only at the Go `/api/v1/search` endpoint. An MP4 extension does not guarantee direct play. There is no automatic transcoding, source fallback, IPTV, health worker, subtitles, video proxy, or high availability. Actual Apple TV playback must be verified on your device and network after deployment.

This V1 release stops at deployment and real-device verification. Future work should be chosen after Apple TV playback is confirmed.

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
2. Open Channels → Online Media → Featured.
3. Select a film and press Play.
4. On the server, run `cd deploy && docker compose logs -f jellyfin media-api`.
5. Confirm `Online Media item`, `Online Media selected source`, `Online Media play URL created`, and `play redirect` with `sourceId` and `targetHost=archive.org`.
6. Confirm the film starts and the Apple TV player reports direct play where available.

Run `docker stats` during playback and watch the `media-api` NET I/O counters and overall VPS outbound traffic. The API should show only small control requests. Sustained MB/s growth means investigate whether Jellyfin is transcoding or proxying instead of Swiftfin fetching the Archive file directly.

## Troubleshooting order

| Symptom | Check |
|---|---|
| Plugin absent | Jellyfin logs, `/config/plugins/OnlineMedia`, `OnlineMedia.dll`, `net10.0` build, Jellyfin 12.1 SDK match |
| Channel opens but is empty | Media API URL and key in plugin settings, `/api/v1/featured`, container DNS and `/health` |
| Selecting a film fails | `/api/v1/media/:id`, `/sources`, candidate count, `directPlay` and `requiresProxy` |
| Play fails | `/play-url`, signed `/play` 302 with `curl -I`, Archive source availability, Swiftfin codec support |
| VPS traffic is high | Jellyfin playback status for transcoding/proxying and whether Swiftfin is using direct play |

Go API supports arbitrary text search at `/api/v1/search`. Swiftfin/Jellyfin Channel V1 does not yet provide arbitrary text input; its UI is Featured, Library, and Search Examples.
