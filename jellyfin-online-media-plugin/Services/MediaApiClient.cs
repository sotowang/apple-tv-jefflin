using System.Diagnostics;
using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.Extensions.Logging;
using OnlineMedia.Models;

namespace OnlineMedia.Services;

public sealed class MediaApiClient
{
    private static readonly HttpClient SharedClient = new(new SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(5) })
    {
        Timeout = Timeout.InfiniteTimeSpan
    };
    private readonly ILogger<MediaApiClient> _logger;

    public MediaApiClient(ILogger<MediaApiClient> logger) => _logger = logger;

    public Task<SearchResponse> SearchAsync(string query, int page, int limit, CancellationToken cancellationToken) =>
        SendAsync<SearchResponse>(HttpMethod.Get, "api/v1/search?q=" + Uri.EscapeDataString(query) + "&page=" + page + "&limit=" + limit, cancellationToken);

    public Task<SearchResponse> FeaturedAsync(int page, int limit, CancellationToken cancellationToken) =>
        SendAsync<SearchResponse>(HttpMethod.Get, "api/v1/featured?page=" + page + "&limit=" + limit, cancellationToken);

    public Task<SearchResponse> LibraryAsync(int page, int limit, CancellationToken cancellationToken) =>
        SendAsync<SearchResponse>(HttpMethod.Get, "api/v1/library?page=" + page + "&limit=" + limit, cancellationToken);

    public Task<MediaRecord> GetMediaAsync(string mediaId, CancellationToken cancellationToken) =>
        SendAsync<MediaRecord>(HttpMethod.Get, "api/v1/media/" + Uri.EscapeDataString(mediaId), cancellationToken);

    public Task<SourcesResponse> GetSourcesAsync(string mediaId, CancellationToken cancellationToken) =>
        SendAsync<SourcesResponse>(HttpMethod.Get, "api/v1/media/" + Uri.EscapeDataString(mediaId) + "/sources", cancellationToken);

    public Task<PlayResponse> CreatePlayUrlAsync(string sourceId, CancellationToken cancellationToken) =>
        SendAsync<PlayResponse>(HttpMethod.Post, "api/v1/sources/" + Uri.EscapeDataString(sourceId) + "/play-url", cancellationToken);

    private async Task<T> SendAsync<T>(HttpMethod method, string endpoint, CancellationToken cancellationToken)
    {
        var cfg = Plugin.Instance?.Configuration ?? throw new MediaApiException(endpoint, null, "", null, "Online Media plugin is not loaded");
        if (!Uri.TryCreate(cfg.MediaApiBaseUrl?.TrimEnd('/') + "/", UriKind.Absolute, out var baseUri) || baseUri.Scheme != Uri.UriSchemeHttps || string.IsNullOrWhiteSpace(cfg.ApiKey))
            throw new MediaApiException(endpoint, null, "", null, "Configure an HTTPS Media API URL and API key");

        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeout.CancelAfter(TimeSpan.FromSeconds(Math.Clamp(cfg.RequestTimeoutSeconds, 1, 60)));
        using var request = new HttpRequestMessage(method, new Uri(baseUri, endpoint));
        request.Headers.Add("X-API-Key", cfg.ApiKey);
        var timer = Stopwatch.StartNew();
        try
        {
            using var response = await SharedClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, timeout.Token).ConfigureAwait(false);
            var requestId = response.Headers.TryGetValues("X-Request-ID", out var values) ? values.FirstOrDefault() : null;
            _logger.LogInformation("Media API response endpoint={Endpoint} status={StatusCode} latencyMs={LatencyMs} requestId={RequestId}", endpoint, (int)response.StatusCode, timer.ElapsedMilliseconds, requestId);
            if (!response.IsSuccessStatusCode)
            {
                await using var stream = await response.Content.ReadAsStreamAsync(timeout.Token).ConfigureAwait(false);
                using var reader = new StreamReader(stream);
                var buffer = new char[512];
                var count = await reader.ReadAsync(buffer.AsMemory(), timeout.Token).ConfigureAwait(false);
                var body = new string(buffer, 0, count);
                _logger.LogWarning("Media API error endpoint={Endpoint} status={StatusCode} body={Body} requestId={RequestId}", endpoint, (int)response.StatusCode, body, requestId);
                throw new MediaApiException(endpoint, response.StatusCode, body, requestId, $"Media API returned {(int)response.StatusCode}");
            }
            var data = await response.Content.ReadFromJsonAsync<T>(cancellationToken: timeout.Token).ConfigureAwait(false);
            return data ?? throw new MediaApiException(endpoint, response.StatusCode, "", requestId, "Media API returned an empty response");
        }
        catch (MediaApiException) { throw; }
        catch (OperationCanceledException ex) when (!cancellationToken.IsCancellationRequested)
        {
            _logger.LogWarning("Media API timeout endpoint={Endpoint} latencyMs={LatencyMs}", endpoint, timer.ElapsedMilliseconds);
            throw new MediaApiException(endpoint, null, "", null, "Media API timeout", ex);
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning(ex, "Media API network error endpoint={Endpoint} latencyMs={LatencyMs}", endpoint, timer.ElapsedMilliseconds);
            throw new MediaApiException(endpoint, null, "", null, "Media API network error", ex);
        }
        catch (JsonException ex)
        {
            _logger.LogWarning(ex, "Media API invalid JSON endpoint={Endpoint}", endpoint);
            throw new MediaApiException(endpoint, null, "", null, "Media API returned invalid JSON", ex);
        }
    }
}
