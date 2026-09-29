# Online Media Jellyfin plugin

Targets Jellyfin 12.1 / .NET 10. Build with `./build.sh` or `dotnet build -c Release`; the script installs the DLL into `../deploy/plugins/OnlineMedia/` for the Compose mount. Restart Jellyfin after copying. Configure the HTTPS Media API Base URL and API Key in Dashboard → Plugins → Online Media. It implements the public `IChannel` and `IRequiresMediaInfoCallback` interfaces and obtains signed remote media URLs from the Go API. It does not parse Internet Archive or manage a database.
