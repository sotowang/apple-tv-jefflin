using MediaBrowser.Controller.Channels;
using MediaBrowser.Controller.Providers;
using MediaBrowser.Model.Channels;
using MediaBrowser.Model.Dto;
using MediaBrowser.Model.Entities;
using MediaBrowser.Model.MediaInfo;
using Microsoft.Extensions.Logging;
using OnlineMedia.Models;
using OnlineMedia.Services;

namespace OnlineMedia.Channels;

public sealed class OnlineChannel : IChannel, IRequiresMediaInfoCallback
{
    private static readonly string[] Examples = ["Night of the Living Dead", "Sherlock Holmes", "Public Domain Movies"];
    private readonly ILogger<OnlineChannel> _logger;
    private readonly MediaApiClient _api;

    public OnlineChannel(ILogger<OnlineChannel> logger, ILogger<MediaApiClient> apiLogger)
    {
        _logger = logger;
        _api = new MediaApiClient(apiLogger);
    }

    public string Name => "在线影视 / Online Media";
    public string Description => "Internet Archive online movies";
    public string DataVersion => "2";
    public string HomePageUrl => "https://archive.org/details/movies";
    public ChannelParentalRating ParentalRating => ChannelParentalRating.GeneralAudience;
    public bool IsEnabledFor(string userId) => true;
    public InternalChannelFeatures GetChannelFeatures() => new() { MediaTypes = [ChannelMediaType.Video], ContentTypes = [ChannelMediaContentType.Movie], MaxPageSize = 20 };
    public Task<DynamicImageResponse> GetChannelImage(ImageType type, CancellationToken cancellationToken) => Task.FromResult(new DynamicImageResponse { HasImage = false });
    public IEnumerable<ImageType> GetSupportedChannelImages() => [];

    public async Task<ChannelItemResult> GetChannelItems(InternalChannelItemQuery query, CancellationToken cancellationToken)
    {
        var folder = query.FolderId ?? "";
        _logger.LogInformation("Online Media channel request folder={FolderId} startIndex={StartIndex}", folder, query.StartIndex);
        if (folder.Length == 0) return new ChannelItemResult { Items = [Folder("library", "Library"), Folder("featured", "Featured"), Folder("examples", "Search Examples")] };
        if (folder == "examples") return new ChannelItemResult { Items = Examples.Select(q => Folder("example:" + q, "Example: " + q)).ToArray() };
        var page = Math.Clamp(query.StartIndex.GetValueOrDefault() / 20 + 1, 1, 100);
        try
        {
            SearchResponse data = folder switch
            {
                "library" => await _api.LibraryAsync(page, 20, cancellationToken).ConfigureAwait(false),
                "featured" => await _api.FeaturedAsync(page, 20, cancellationToken).ConfigureAwait(false),
                _ when folder.StartsWith("example:", StringComparison.Ordinal) => await _api.SearchAsync(folder[8..], page, 20, cancellationToken).ConfigureAwait(false),
                _ => new SearchResponse([])
            };
            var items = (data.Items ?? []).Select(m => new ChannelItemInfo { Id = m.Id, Name = m.Title, Type = ChannelItemType.Media, MediaType = ChannelMediaType.Video, ContentType = ChannelMediaContentType.Movie, ProductionYear = m.Year > 0 ? m.Year : null, Overview = m.Overview }).ToArray();
            _logger.LogInformation("Online Media channel response folder={FolderId} itemCount={ItemCount}", folder, items.Length);
            return new ChannelItemResult { Items = items, TotalRecordCount = items.Length < 20 ? (page - 1) * 20 + items.Length : null };
        }
        catch (MediaApiException ex)
        {
            _logger.LogError(ex, "Online Media channel failed folder={FolderId} endpoint={Endpoint} status={StatusCode} requestId={RequestId}", folder, ex.Endpoint, ex.StatusCode, ex.RequestId);
            throw;
        }
    }
    private static ChannelItemInfo Folder(string id, string name) => new() { Id = id, Name = name, Type = ChannelItemType.Folder, FolderType = ChannelFolderType.Container };

    public async Task<IEnumerable<MediaSourceInfo>> GetChannelItemMediaInfo(string id, CancellationToken cancellationToken)
    {
        try
        {
            var detail = await _api.GetMediaAsync(id, cancellationToken).ConfigureAwait(false);
            var sourceResponse = await _api.GetSourcesAsync(id, cancellationToken).ConfigureAwait(false);
            var sources = sourceResponse.Sources ?? [];
            _logger.LogInformation("Online Media item mediaId={MediaId} title={Title} sourceCount={SourceCount}", id, detail.Title, sources.Length);
            // The Go API orders source candidates. V1 deliberately exposes one candidate and has no automatic fallback.
            var selected = sources.FirstOrDefault(s => s.DirectPlay && !s.RequiresProxy);
            if (selected is null) { _logger.LogWarning("Online Media has no direct-play candidate mediaId={MediaId}", id); return []; }
            _logger.LogInformation("Online Media selected source mediaId={MediaId} sourceId={SourceId} container={Container}", id, selected.Id, selected.Container);
            var signed = await _api.CreatePlayUrlAsync(selected.Id, cancellationToken).ConfigureAwait(false);
            if (!Uri.TryCreate(signed.Url, UriKind.Absolute, out var playUri) || playUri.Scheme != Uri.UriSchemeHttps)
                throw new MediaApiException("api/v1/sources/:id/play-url", null, "", null, "Media API returned a non-HTTPS play URL");
            _logger.LogInformation("Online Media play URL created mediaId={MediaId} sourceId={SourceId} host={Host} path={Path}", id, selected.Id, playUri.Host, playUri.AbsolutePath);
            return [new MediaSourceInfo { Id = selected.Id, Name = selected.FileName, Path = signed.Url, Protocol = MediaProtocol.Http, IsRemote = true, Container = selected.Container, SupportsDirectPlay = true, SupportsDirectStream = false, SupportsTranscoding = false, SupportsProbing = false }];
        }
        catch (MediaApiException ex)
        {
            _logger.LogError(ex, "Online Media playback failed mediaId={MediaId} endpoint={Endpoint} status={StatusCode} requestId={RequestId}", id, ex.Endpoint, ex.StatusCode, ex.RequestId);
            throw;
        }
    }
}
