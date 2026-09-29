#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
if command -v dotnet >/dev/null; then
  dotnet build -c Release
else
  docker run --rm -v "$PWD:/src" -w /src mcr.microsoft.com/dotnet/sdk:10.0 dotnet build -c Release
fi
mkdir -p ../deploy/plugins/OnlineMedia
cp bin/Release/net10.0/OnlineMedia.dll ../deploy/plugins/OnlineMedia/
