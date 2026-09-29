using System.Net;

namespace OnlineMedia.Services;

public sealed class MediaApiException : Exception
{
    public MediaApiException(string endpoint, HttpStatusCode? statusCode, string responseBody, string? requestId, string message, Exception? inner = null)
        : base(message, inner)
    {
        Endpoint = endpoint;
        StatusCode = statusCode;
        ResponseBody = responseBody;
        RequestId = requestId;
    }
    public string Endpoint { get; }
    public HttpStatusCode? StatusCode { get; }
    public string ResponseBody { get; }
    public string? RequestId { get; }
}
