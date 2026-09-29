namespace OnlineMedia.Models;

public sealed record SourceRecord(string Id, string FileName, string Container, bool DirectPlay, bool RequiresProxy, string? Quality, string? VideoCodec, string? AudioCodec);
public sealed record SourcesResponse(SourceRecord[] Sources);
