using MediaBrowser.Controller;
using MediaBrowser.Controller.Channels;
using MediaBrowser.Controller.Plugins;
using Microsoft.Extensions.DependencyInjection;
using OnlineMedia.Channels;
using OnlineMedia.Services;

namespace OnlineMedia;

public sealed class PluginServiceRegistrator : IPluginServiceRegistrator
{
    public void RegisterServices(IServiceCollection serviceCollection, IServerApplicationHost applicationHost)
    {
        serviceCollection.AddSingleton<MediaApiClient>();
        serviceCollection.AddSingleton<IChannel, OnlineChannel>();
    }
}
