#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
run_smoke_override="${RUN_SMOKE_TEST:-}"
set -a
source .env
set +a
if [[ -n "$run_smoke_override" ]]; then RUN_SMOKE_TEST="$run_smoke_override"; fi
test -f plugins/OnlineMedia/OnlineMedia.dll || { echo 'Build the Online Media plugin first' >&2; exit 1; }
docker compose build
docker compose up -d
docker compose ps
for service in postgres media-api jellyfin; do
  cid="$(docker compose ps -q "$service")"
  test -n "$cid" || { echo "$service container missing" >&2; exit 1; }
  status="$(docker inspect -f '{{.State.Health.Status}}' "$cid")"
  for i in $(seq 1 60); do
    if [ "$status" = healthy ]; then break; fi
    sleep 2
    status="$(docker inspect -f '{{.State.Health.Status}}' "$cid")"
  done
  test "$status" = healthy || { echo "$service health: $status" >&2; exit 1; }
  echo "[OK] $service healthy"
done
caddy_id="$(docker compose ps -q caddy)"
test -n "$caddy_id" && test "$(docker inspect -f '{{.State.Running}}' "$caddy_id")" = true || { echo 'caddy is not running' >&2; exit 1; }
echo '[OK] caddy running'
health="$(curl --fail --silent --show-error --connect-timeout 10 --max-time 30 "${PUBLIC_MEDIA_BASE_URL%/}/health")"
[[ "$health" == *'"status":"ok"'* ]] || { echo 'public Media API health failed' >&2; exit 1; }
echo '[OK] public HTTPS health'
docker compose --profile catalog run --rm catalog-sync
echo '[OK] online movie catalog synced'
if [[ "${RUN_SMOKE_TEST:-false}" == true ]]; then
  MEDIA_API_URL="$PUBLIC_MEDIA_BASE_URL" ./smoke-test.sh
fi
