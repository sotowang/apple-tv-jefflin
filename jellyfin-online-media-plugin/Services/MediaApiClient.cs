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
    private int _healthChecked;

    public MediaApiClient(ILogger<MediaApiClient> logger) => _logger = logger;

    public bool IsConfigured => TryGetBaseUri(out _);

    private static bool TryGetBaseUri(out Uri? baseUri)
    {
        var value = Plugin.Instance?.Configuration.MediaApiBaseUrl;
        return Uri.TryCreate(value?.TrimEnd('/') + "/", UriKind.Absolute, out baseUri)
            && baseUri.Scheme == Uri.UriSchemeHttps;
    }

    public async Task CheckHealthOnceAsync(CancellationToken cancellationToken)
    {
        if (!TryGetBaseUri(out var baseUri))
        {
            _logger.LogError("Online Media: Media API URL is not configured");
            return;
        }
        if (Interlocked.Exchange(ref _healthChecked, 1) != 0) return;
        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeout.CancelAfter(TimeSpan.FromSeconds(Math.Clamp(Plugin.Instance?.Configuration.RequestTimeoutSeconds ?? 10, 1, 60)));
        try
        {
            using var response = await SharedClient.GetAsync(new Uri(baseUri!, "health"), timeout.Token).ConfigureAwait(false);
            if (response.IsSuccessStatusCode)
                _logger.LogInformation("Media API health check passed host={Host}", baseUri!.Host);
            else
                _logger.LogWarning("Media API health check failed host={Host} reason=status upstreamStatus={UpstreamStatus}", baseUri!.Host, (int)response.StatusCode);
        }
        catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
        {
            _logger.LogWarning("Media API health check failed host={Host} reason=timeout", baseUri!.Host);
        }
        catch (HttpRequestException ex)
        {
            _logger.LogWarning("Media API health check failed host={Host} reason=network errorType={ErrorType}", baseUri!.Host, ex.GetType().Name);
        }
    }

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
        if (!TryGetBaseUri(out var baseUri))
        {
            _logger.LogError("Online Media: Media API URL is not configured");
            throw new MediaApiException(endpoint, null, "", null, "Online Media: Media API URL is not configured");
        }
        if (string.IsNullOrWhiteSpace(cfg.ApiKey))
        {
            _logger.LogError("Online Media: Media API key is not configured");
            throw new MediaApiException(endpoint, null, "", null, "Online Media: Media API key is not configured");
        }

        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        timeout.CancelAfter(TimeSpan.FromSeconds(Math.Clamp(cfg.RequestTimeoutSeconds, 1, 60)));
        using var request = new HttpRequestMessage(method, new Uri(baseUri!, endpoint));
        request.Headers.Add("X-API-Key", cfg.ApiKey);
        var timer = Stopwatch.StartNew();
        try
        {
            using var response = await SharedClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, timeout.Token).ConfigureAwait(false);
            var requestId = response.Headers.TryGetValues("X-Request-ID", out var values) ? values.FirstOrDefault() : null;
            _logger.LogInformation("Media API response endpoint={Endpoint} status={StatusCode} latencyMs={LatencyMs} requestId={RequestId}", endpoint, (int)response.StatusCode, timer.ElapsedMilliseconds, requestId);
            if (!response.IsSuccessStatusCode)
            {
                _logger.LogWarning("Media API error endpoint={Endpoint} status={StatusCode} requestId={RequestId}", endpoint, (int)response.StatusCode, requestId);
                throw new MediaApiException(endpoint, response.StatusCode, "", requestId, $"Media API returned {(int)response.StatusCode}");
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
