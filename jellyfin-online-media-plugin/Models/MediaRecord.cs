namespace OnlineMedia.Models;

public sealed record MediaRecord(string Id, string Title, int Year, string? Overview, string? RightsStatus);
public sealed record SearchResponse(MediaRecord[] Items);
