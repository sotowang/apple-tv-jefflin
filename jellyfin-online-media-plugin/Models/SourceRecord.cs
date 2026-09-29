namespace OnlineMedia.Models;

public sealed record SourceRecord(string Id, string FileName, string Container, bool DirectPlay, bool RequiresProxy);
public sealed record SourcesResponse(SourceRecord[] Sources);
