using System.Net.Http.Json;
using System.Text.Json.Serialization;
using MediaBrowser.Controller.Channels;
using MediaBrowser.Controller.Providers;
using MediaBrowser.Model.Channels;
using MediaBrowser.Model.Dto;
using MediaBrowser.Model.Entities;
using MediaBrowser.Model.MediaInfo;


namespace OnlineMedia;

public sealed class OnlineChannel : IChannel, IRequiresMediaInfoCallback
{
    private static readonly string[] Searches = ["Night of the Living Dead", "Sherlock Holmes", "Public Domain Movies"];
    public string Name => "在线影视 / Online Media";
    public string Description => "Internet Archive online movies";
    public string DataVersion => "1";
    public string HomePageUrl => "https://archive.org/details/movies";
    public ChannelParentalRating ParentalRating => ChannelParentalRating.GeneralAudience;
    public bool IsEnabledFor(string userId) => true;
    public InternalChannelFeatures GetChannelFeatures() => new() { MediaTypes = [ChannelMediaType.Video], ContentTypes = [ChannelMediaContentType.Movie], MaxPageSize = 50 };
    public Task<DynamicImageResponse> GetChannelImage(ImageType type, CancellationToken cancellationToken) => Task.FromResult(new DynamicImageResponse { HasImage = false });
    public IEnumerable<ImageType> GetSupportedChannelImages() => [];
    private static HttpClient Client()
    {
        var cfg = Plugin.Instance?.Configuration ?? throw new InvalidOperationException("Plugin not loaded");
        if (!Uri.TryCreate(cfg.MediaApiBaseUrl, UriKind.Absolute, out var uri) || uri.Scheme != Uri.UriSchemeHttps) throw new InvalidOperationException("Configure HTTPS Media API URL");
        if (string.IsNullOrWhiteSpace(cfg.ApiKey)) throw new InvalidOperationException("Configure Media API key");
        var client = new HttpClient { BaseAddress = new Uri(cfg.MediaApiBaseUrl.TrimEnd('/') + "/"), Timeout = TimeSpan.FromSeconds(Math.Clamp(cfg.RequestTimeoutSeconds, 1, 60)) };
        client.DefaultRequestHeaders.Add("X-API-Key", cfg.ApiKey);
        return client;
    }
    public async Task<ChannelItemResult> GetChannelItems(InternalChannelItemQuery query, CancellationToken cancellationToken)
    {
        if (string.IsNullOrEmpty(query.FolderId)) return new ChannelItemResult { Items = [Folder("movies", "Movies"), Folder("search", "Search")] };
        if (query.FolderId == "search") return new ChannelItemResult { Items = Searches.Select(q => Folder("query:" + q, q)).ToArray() };
        var search = query.FolderId == "movies" ? "movie" : query.FolderId.StartsWith("query:", StringComparison.Ordinal) ? query.FolderId[6..] : "";
        if (search.Length == 0) return new ChannelItemResult();
        var page = Math.Clamp(query.StartIndex.GetValueOrDefault() / 20 + 1, 1, 100);
        using var client = Client();
        using var response = await client.GetAsync("api/v1/search?q=" + Uri.EscapeDataString(search) + "&page=" + page + "&limit=20", cancellationToken).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
        var data = await response.Content.ReadFromJsonAsync<ItemsResponse>(cancellationToken).ConfigureAwait(false);
        var items = (data?.Items ?? []).Select(m => new ChannelItemInfo { Id = m.Id, Name = m.Title, Type = ChannelItemType.Media, MediaType = ChannelMediaType.Video, ContentType = ChannelMediaContentType.Movie, ProductionYear = m.Year > 0 ? m.Year : null, Overview = m.Overview }).ToArray();
        return new ChannelItemResult { Items = items, TotalRecordCount = items.Length < 20 ? (page - 1) * 20 + items.Length : null };
    }
    private static ChannelItemInfo Folder(string id, string name) => new() { Id = id, Name = name, Type = ChannelItemType.Folder, FolderType = ChannelFolderType.Container };
    public async Task<IEnumerable<MediaSourceInfo>> GetChannelItemMediaInfo(string id, CancellationToken cancellationToken)
    {
        using var client = Client();
        var mediaId = Uri.EscapeDataString(id);
        using var detail = await client.GetAsync("api/v1/media/" + mediaId, cancellationToken).ConfigureAwait(false);
        detail.EnsureSuccessStatusCode();
        using var response = await client.GetAsync("api/v1/media/" + mediaId + "/sources", cancellationToken).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
        var data = await response.Content.ReadFromJsonAsync<SourcesResponse>(cancellationToken).ConfigureAwait(false);
        var result = new List<MediaSourceInfo>();
        foreach (var source in data?.Sources ?? [])
        {
            if (!source.DirectPlay || source.RequiresProxy) continue;
            using var play = await client.PostAsync("api/v1/sources/" + Uri.EscapeDataString(source.Id) + "/play-url", null, cancellationToken).ConfigureAwait(false);
            play.EnsureSuccessStatusCode();
            var signed = await play.Content.ReadFromJsonAsync<PlayResponse>(cancellationToken).ConfigureAwait(false);
            if (signed?.Url is null) continue;
            result.Add(new MediaSourceInfo { Id = source.Id, Name = source.FileName, Path = signed.Url, Protocol = MediaProtocol.Http, IsRemote = true, Container = source.Container, SupportsDirectPlay = true, SupportsDirectStream = false, SupportsTranscoding = false, SupportsProbing = false });
        }
        return result;
    }
    private sealed record MediaRecord(string Id, string Title, int Year, string? Overview);
    private sealed record ItemsResponse([property: JsonPropertyName("items")] MediaRecord[] Items);
    private sealed record SourceRecord(string Id, string FileName, string Container, bool DirectPlay, bool RequiresProxy);
    private sealed record SourcesResponse([property: JsonPropertyName("sources")] SourceRecord[] Sources);
    private sealed record PlayResponse(string Url);
}
