#!/usr/bin/env bash
set -euo pipefail
: "${MEDIA_API_URL:?set MEDIA_API_URL}"
: "${API_KEY:?set API_KEY}"
command -v jq >/dev/null || { echo 'jq is required' >&2; exit 1; }
base="${MEDIA_API_URL%/}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
api_get() {
  curl -fsS --connect-timeout 5 --max-time 30 -H "X-API-Key: ${API_KEY}" "$base$1"
}
health="$(curl -fsS --connect-timeout 5 --max-time 10 "$base/health")"
[[ "$(jq -r '.status' <<<"$health")" == ok ]] || { echo 'health failed' >&2; exit 1; }
echo '[OK] health'
featured="$(api_get '/api/v1/featured?page=1&limit=20')"
media_id="$(jq -r '.items[0].id // empty' <<<"$featured")"
[[ -n "$media_id" ]] || { echo 'featured has no media' >&2; exit 1; }
echo '[OK] featured'
media="$(api_get "/api/v1/media/${media_id}")"
[[ "$(jq -r '.id' <<<"$media")" != null ]] || { echo 'media failed' >&2; exit 1; }
echo '[OK] media'
sources="$(api_get "/api/v1/media/${media_id}/sources")"
source_id="$(jq -r '[.sources[] | select(.directPlay == true and .requiresProxy == false)][0].id // empty' <<<"$sources")"
[[ -n "$source_id" ]] || { echo 'no direct-play source' >&2; exit 1; }
echo '[OK] source'
play="$(curl -fsS --connect-timeout 5 --max-time 30 -X POST -H "X-API-Key: ${API_KEY}" "$base/api/v1/sources/${source_id}/play-url")"
signed_url="$(jq -r '.url // empty' <<<"$play")"
[[ "$signed_url" == "$base/play/"* ]] || { echo 'signed play URL has unexpected origin' >&2; exit 1; }
echo '[OK] signed play'
status="$(curl -sS -I --connect-timeout 5 --max-time 30 --max-redirs 0 -o "$tmp/headers" -w '%{http_code}' "$signed_url")"
[[ "$status" == 302 ]] || { echo "play redirect status: $status" >&2; exit 1; }
location="$(tr -d '\r' < "$tmp/headers" | sed -n 's/^[Ll]ocation: //p' | tail -1)"
if [[ "$location" =~ ^https://([^/:?]+) ]]; then
  host="${BASH_REMATCH[1]}"
else
  echo 'redirect Location is not HTTPS' >&2; exit 1
fi
[[ "$host" == archive.org || "$host" == *.archive.org ]] || { echo "unexpected redirect host: $host" >&2; exit 1; }
echo "[OK] redirect 302 -> $host"
echo 'SMOKE TEST PASSED'
