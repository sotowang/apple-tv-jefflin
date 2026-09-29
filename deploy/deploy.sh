#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
set -a
source .env
set +a
docker compose build
docker compose up -d
docker compose ps
for service in postgres media-api jellyfin; do
  cid="$(docker compose ps -q "$service")"
  test -n "$cid"
  status="$(docker inspect -f '{{.State.Health.Status}}' "$cid")"
  for i in $(seq 1 30); do
    if [ "$status" = healthy ]; then break; fi
    sleep 2
    status="$(docker inspect -f '{{.State.Health.Status}}' "$cid")"
  done
  test "$status" = healthy || { echo "$service health: $status" >&2; exit 1; }
done
curl --fail --silent --show-error "${PUBLIC_MEDIA_BASE_URL}/health"
printf '\n'
