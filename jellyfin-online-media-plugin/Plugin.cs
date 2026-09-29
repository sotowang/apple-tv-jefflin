using MediaBrowser.Common.Configuration;
using MediaBrowser.Common.Plugins;
using MediaBrowser.Model.Plugins;
using MediaBrowser.Model.Serialization;
using Microsoft.Extensions.Logging;

namespace OnlineMedia;

public sealed class Plugin : BasePlugin<PluginConfiguration>, IHasWebPages
{
    public Plugin(IApplicationPaths paths, IXmlSerializer serializer, ILogger<Plugin> logger) : base(paths, serializer)
    {
        Instance = this;
        logger.LogInformation("Online Media plugin initialized");
        logger.LogInformation("Media API base URL {ConfigurationState}", Uri.TryCreate(Configuration.MediaApiBaseUrl, UriKind.Absolute, out var url) && url.Scheme == Uri.UriSchemeHttps ? "configured" : "not configured");
    }
    public override string Name => "Online Media";
    public override Guid Id => Guid.Parse("e75b5cdd-c166-497f-b9b4-f425cf80e7a4");
    public static Plugin? Instance { get; private set; }
    public IEnumerable<PluginPageInfo> GetPages() => new[] { new PluginPageInfo { Name = Name, EmbeddedResourcePath = "OnlineMedia.Configuration.configPage.html" } };
}
