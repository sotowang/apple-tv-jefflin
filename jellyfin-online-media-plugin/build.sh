#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
if command -v dotnet >/dev/null; then
  dotnet publish -c Release -o "$PWD/bin/plugin-publish"
else
  docker run --rm -v "$PWD:/src" -w /src mcr.microsoft.com/dotnet/sdk:10.0 dotnet publish -c Release -o /src/bin/plugin-publish
fi
plugin_dir="../deploy/plugins/OnlineMedia"
mkdir -p "$plugin_dir"
find "$plugin_dir" -maxdepth 1 -type f -delete
find bin/plugin-publish -maxdepth 1 -type f \( -name '*.dll' -o -name '*.json' \) ! -name 'Jellyfin.*' ! -name 'MediaBrowser.*' ! -name 'Microsoft.*' ! -name 'System.*' -exec cp {} "$plugin_dir/" \;
test -f "$plugin_dir/OnlineMedia.dll"
echo 'Plugin output:'
echo 'deploy/plugins/OnlineMedia/OnlineMedia.dll'
