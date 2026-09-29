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
INSERT INTO media(type,title,original_title,year,overview,poster_url,backdrop_url,provider_id,external_id,license_url,rights)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT(provider_id,external_id) DO UPDATE SET type=excluded.type,title=excluded.title,original_title=excluded.original_title,year=excluded.year,overview=excluded.overview,poster_url=excluded.poster_url,backdrop_url=excluded.backdrop_url,license_url=excluded.license_url,rights=excluded.rights,updated_at=now()
RETURNING *;
-- name: GetSourcesByMediaID :many
SELECT * FROM sources WHERE media_id=$1 AND status='available' ORDER BY direct_play DESC, file_name;
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
