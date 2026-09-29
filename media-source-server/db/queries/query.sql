-- name: ListProviders :many
SELECT * FROM providers WHERE enabled = true ORDER BY priority DESC, name;
-- name: GetProviderByID :one
SELECT * FROM providers WHERE id=$1 AND enabled=true;
-- name: GetProviderByName :one
SELECT * FROM providers WHERE name = $1 AND enabled = true;
-- name: GetMediaByID :one
SELECT * FROM media WHERE id = $1;
-- name: GetMediaByExternalID :one
SELECT m.* FROM media m JOIN providers p ON p.id=m.provider_id WHERE p.name=$1 AND m.external_id=$2;
-- name: UpsertMedia :one
INSERT INTO media(type,title,original_title,year,overview,poster_url,backdrop_url,provider_id,external_id,license_url,rights,release_date,original_language,tmdb_id,season_count,episode_count)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
ON CONFLICT(provider_id,external_id) DO UPDATE SET type=excluded.type,title=excluded.title,original_title=excluded.original_title,year=excluded.year,overview=excluded.overview,poster_url=excluded.poster_url,backdrop_url=excluded.backdrop_url,license_url=excluded.license_url,rights=excluded.rights,release_date=excluded.release_date,original_language=excluded.original_language,tmdb_id=excluded.tmdb_id,season_count=excluded.season_count,episode_count=excluded.episode_count,updated_at=now()
RETURNING *;
-- name: UpsertSeason :one
INSERT INTO seasons(media_id,season_number,name,overview,poster_url,air_date,episode_count,external_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(media_id,season_number) DO UPDATE SET name=excluded.name,overview=excluded.overview,poster_url=excluded.poster_url,air_date=excluded.air_date,episode_count=excluded.episode_count,external_id=excluded.external_id,updated_at=now()
RETURNING *;
-- name: GetSeasonsByMediaID :many
SELECT * FROM seasons WHERE media_id=$1 ORDER BY season_number;
-- name: UpsertEpisode :one
INSERT INTO episodes(season_id,episode_number,name,overview,air_date,runtime_minutes,still_url,external_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8)
ON CONFLICT(season_id,episode_number) DO UPDATE SET name=excluded.name,overview=excluded.overview,air_date=excluded.air_date,runtime_minutes=excluded.runtime_minutes,still_url=excluded.still_url,external_id=excluded.external_id,updated_at=now()
RETURNING *;
-- name: GetEpisodesBySeasonID :many
SELECT * FROM episodes WHERE season_id=$1 ORDER BY episode_number;
-- name: GetSeasonByMediaAndNumber :one
SELECT * FROM seasons WHERE media_id=$1 AND season_number=$2;
-- name: GetSourcesByMediaID :many
SELECT * FROM sources WHERE media_id=$1 AND status='available' ORDER BY direct_play DESC, CASE container WHEN 'mp4' THEN 0 WHEN 'm4v' THEN 1 WHEN 'mov' THEN 2 WHEN 'webm' THEN 3 WHEN 'mkv' THEN 4 ELSE 5 END, CASE WHEN quality ~ '^[0-9]+p$' THEN substring(quality from '^[0-9]+')::int ELSE 0 END DESC, file_name;
-- name: GetSourceByID :one
SELECT * FROM sources WHERE id=$1 AND status='available';
-- name: UpsertSource :one
INSERT INTO sources(media_id,provider_id,external_id,file_name,original_url,quality,container,video_codec,audio_codec,bitrate,direct_play,requires_proxy,status)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'available')
ON CONFLICT(provider_id,external_id,file_name) DO UPDATE SET media_id=excluded.media_id,original_url=excluded.original_url,quality=excluded.quality,container=excluded.container,video_codec=excluded.video_codec,audio_codec=excluded.audio_codec,bitrate=excluded.bitrate,direct_play=excluded.direct_play,requires_proxy=excluded.requires_proxy,status='available',updated_at=now()
RETURNING *;
-- name: AddLibraryItem :exec
INSERT INTO library_items(media_id) VALUES($1) ON CONFLICT(media_id) DO NOTHING;
-- name: RemoveLibraryItem :exec
DELETE FROM library_items WHERE media_id=$1;
-- name: ListLibraryItems :many
SELECT m.* FROM library_items li JOIN media m ON m.id=li.media_id ORDER BY li.created_at DESC LIMIT $1 OFFSET $2;
