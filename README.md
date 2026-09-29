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
cd deploy
docker compose up -d --build
./deploy.sh
```

`deploy.sh` builds, starts, shows container status, waits for PostgreSQL, API, and Jellyfin health, then calls `https://<media host>/health`. It needs working DNS and ports 80/443 for Caddy certificate issuance. Verify manually:

```sh
curl https://media.example.com/health
# {"status":"ok"}
docker compose ps
```

The API runs `001_initial.sql` at startup before serving requests. This idempotent SQL creates all V1 tables and the Archive provider. For an explicit migration run, use `docker compose run --rm media-api /app/migrate`. Schema changes in later versions must use new numbered migrations; do not edit a migration already applied to production.

## Jellyfin and Apple TV

1. Open `https://jellyfin.example.com`, create the Jellyfin administrator, and finish setup.
2. Build the plugin on the VPS or locally with .NET SDK 10 or Docker:

   ```sh
   ./jellyfin-online-media-plugin/build.sh
   ```

   This runs `dotnet build -c Release` and copies `OnlineMedia.dll` into `deploy/plugins/OnlineMedia/`. If built locally, copy that DLL to the same path on the VPS. This path is mounted at `/config/plugins/OnlineMedia` in Jellyfin.
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

In non-production environments (`APP_ENV != production`), authenticated `GET /api/v1/debug/media/:id` returns media, sources, selected candidate, and resolved URL **host only**. The route is absent in production. The Go API does not serve video Range data: after the signed `/play` redirect, the client communicates directly with Archive/CDN, which handles Range requests. No video bytes pass through this VPS.

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

Provider logic lives behind the Go `Provider` interface. V2 can add legal IPTV/VOD providers, live TV and EPG, health checks, ranking, favorites, and episode mapping. PostgreSQL remains the system of record; add Redis only if multiple API instances or shared short lived state require it.
