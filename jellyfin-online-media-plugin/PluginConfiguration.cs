using MediaBrowser.Model.Plugins;
namespace OnlineMedia;
public sealed class PluginConfiguration : BasePluginConfiguration
{
    public string MediaApiBaseUrl { get; set; } = "";
    public string ApiKey { get; set; } = "";
    public int RequestTimeoutSeconds { get; set; } = 10;
}
