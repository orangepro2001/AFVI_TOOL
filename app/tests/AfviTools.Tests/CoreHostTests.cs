using AfviTools.Localization;
using AfviTools.Services;

namespace AfviTools.Tests;

public class CoreHostTests
{
    [Fact]
    public async Task CoreStartsWritesDefaultMachineAndLocksNetwork()
    {
        var dir = Path.Combine(Path.GetTempPath(), "afvi-core-" + Guid.NewGuid().ToString("N")[..8]);
        try
        {
            using var host = await CoreHost.Start(Path.Combine(dir, "config.json"), false);
            var health = await host.Client.Health();
            Assert.False(health.AllowNetworkWrite);
            var cfg = await host.Client.GetConfig();
            var m = Assert.Single(cfg.Machines);
            Assert.Equal("AFVI 13", m.Name);
            Assert.Equal("192.168.1.61", m.Fm1.Host);
            Assert.Equal("192.168.1.62", m.Fm2.Host);
            Assert.Equal("192.168.1.63", m.Bm.Host);
            Assert.Equal("pixel", cfg.Settings.SmbUser);
            Assert.True(File.Exists(Path.Combine(dir, "config.json")), "the default machine must be written on first start");
        }
        finally { try { Directory.Delete(dir, true); } catch (IOException) { } }
    }

    [Fact]
    public async Task CoreExitsWhenTheParentGoesAway()
    {
        var dir = Path.Combine(Path.GetTempPath(), "afvi-core-" + Guid.NewGuid().ToString("N")[..8]);
        var host = await CoreHost.Start(Path.Combine(dir, "config.json"), false);
        var address = host.Client.BaseAddress;
        host.Dispose(); // closes stdin, like a crashed GUI would
        using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(2) };
        for (var i = 0; i < 20; i++)
        {
            try { await http.GetAsync(new Uri(address, "/v1/health")); await Task.Delay(200); }
            catch (HttpRequestException) { goto gone; }
        }
        Assert.Fail("the core is still serving after its parent closed stdin");
    gone:
        try { Directory.Delete(dir, true); } catch (IOException) { }
    }
}
