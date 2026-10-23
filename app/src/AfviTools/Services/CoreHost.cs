using System.Diagnostics;
using System.Text.Json;
using AfviTools.Localization;

namespace AfviTools.Services;

/// <summary>
/// Starts the Go core as a child process and owns its life: the core prints
/// one JSON line with its random loopback port and token, and it exits by
/// itself when our end of its stdin closes (even if we crash).
/// </summary>
public sealed class CoreHost : IDisposable
{
    private Process? _proc;

    public CoreClient Client { get; private set; } = null!;
    public string Version { get; private set; } = "";

    private const string EmbeddedName = "afvi-core.bin";

    /// <summary>
    /// Where the core is: AFVI_CORE, next to the program, the dev build folder, and last the copy carried inside
    /// the program (unpacked once per version into the user's data folder, so the user never sees it).
    /// </summary>
    public static string? FindCore()
    {
        var exe = OperatingSystem.IsWindows() ? "afvi-core.exe" : "afvi-core";
        var env = Environment.GetEnvironmentVariable("AFVI_CORE");
        if (!string.IsNullOrEmpty(env) && File.Exists(env)) return env;
        var dir = AppContext.BaseDirectory;
        // next to the app, or in core/; dev: <repo>/bin from <repo>/app/src/AfviTools/bin/<cfg>/<tfm>/
        foreach (var rel in new[] { "", "core", "../../../../../../bin" })
        {
            var p = Path.GetFullPath(Path.Combine(dir, rel, exe));
            if (File.Exists(p)) return p;
        }
        return ExtractEmbedded();
    }

    public static bool HasEmbeddedCore => typeof(CoreHost).Assembly.GetManifestResourceInfo(EmbeddedName) != null;

    /// <summary>Unpacks the embedded core to <c>&lt;local app data&gt;/AFVI_TOOLS/core/afvi-core-&lt;hash&gt;</c>; reuses it when it is already there.</summary>
    public static string? ExtractEmbedded()
    {
        var asm = typeof(CoreHost).Assembly;
        using var src = asm.GetManifestResourceStream(EmbeddedName);
        if (src == null) return null;

        var dir = Path.Combine(UiSettings.DataDirectory(), "core");
        Directory.CreateDirectory(dir);
        var hash = Convert.ToHexString(System.Security.Cryptography.SHA256.HashData(src)).ToLowerInvariant()[..16];
        src.Position = 0;
        var target = Path.Combine(dir, $"afvi-core-{hash}" + (OperatingSystem.IsWindows() ? ".exe" : ""));
        if (File.Exists(target) && new FileInfo(target).Length == src.Length) return target;

        // write beside it, then move into place, so two starting instances never see a half written file
        var tmp = target + "." + Environment.ProcessId + ".tmp";
        using (var dst = File.Create(tmp)) src.CopyTo(dst);
        if (!OperatingSystem.IsWindows())
            File.SetUnixFileMode(tmp, UnixFileMode.UserRead | UnixFileMode.UserWrite | UnixFileMode.UserExecute |
                                      UnixFileMode.GroupRead | UnixFileMode.GroupExecute | UnixFileMode.OtherRead | UnixFileMode.OtherExecute);
        try { File.Move(tmp, target, overwrite: true); }
        catch (IOException) { File.Delete(tmp); if (!File.Exists(target)) throw; } // another instance won the race

        // old versions, best effort (one may still be running)
        foreach (var old in Directory.EnumerateFiles(dir, "afvi-core-*"))
        {
            if (string.Equals(old, target, StringComparison.OrdinalIgnoreCase)) continue;
            try { File.Delete(old); } catch (Exception ex) when (ex is IOException or UnauthorizedAccessException) { }
        }
        return target;
    }

    public static async Task<CoreHost> Start(string? configPath, bool allowNetworkWrite, CancellationToken ct = default)
    {
        var path = FindCore() ?? throw new FileNotFoundException(Loc.Instance["err_core_missing"]);
        var psi = new ProcessStartInfo(path)
        {
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        psi.ArgumentList.Add("-watch-stdin");
        if (!string.IsNullOrEmpty(configPath)) { psi.ArgumentList.Add("-config"); psi.ArgumentList.Add(configPath); }
        if (allowNetworkWrite) psi.ArgumentList.Add("-allow-network-write");

        var host = new CoreHost();
        host._proc = Process.Start(psi) ?? throw new InvalidOperationException("cannot start afvi-core");
        host._proc.ErrorDataReceived += (_, e) => { if (e.Data != null) Trace.WriteLine("afvi-core: " + e.Data); };
        host._proc.BeginErrorReadLine();

        using var timeout = CancellationTokenSource.CreateLinkedTokenSource(ct);
        timeout.CancelAfter(TimeSpan.FromSeconds(15));
        string? line;
        try { line = await host._proc.StandardOutput.ReadLineAsync(timeout.Token); }
        catch (OperationCanceledException)
        {
            host.Dispose();
            throw new TimeoutException(Loc.Instance["err_core_timeout"]);
        }
        if (line == null)
        {
            var code = host._proc.HasExited ? host._proc.ExitCode : -1;
            host.Dispose();
            throw new InvalidOperationException(Loc.Instance.Format("err_core_exit", code));
        }
        using var doc = JsonDocument.Parse(line);
        var root = doc.RootElement;
        var port = root.GetProperty("port").GetInt32();
        host.Version = root.GetProperty("version").GetString() ?? "";
        host.Client = new CoreClient(new Uri($"http://127.0.0.1:{port}"), root.GetProperty("token").GetString()!);
        return host;
    }

    public void Dispose()
    {
        Client?.Dispose();
        if (_proc == null) return;
        try
        {
            _proc.StandardInput.Close(); // the core exits when stdin closes
            if (!_proc.WaitForExit(1500)) _proc.Kill(entireProcessTree: true);
        }
        catch (InvalidOperationException) { }
        _proc.Dispose();
        _proc = null;
    }
}
