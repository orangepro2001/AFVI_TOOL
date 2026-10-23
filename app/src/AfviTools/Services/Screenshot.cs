using Avalonia;
using Avalonia.Controls;
using Avalonia.Media.Imaging;
using Avalonia.Threading;

namespace AfviTools.Services;

/// <summary>
/// Renders a window to a PNG so the Windows 2000 look can be checked without a human
/// (--screenshot=&lt;file&gt; [--shot-delay=ms]); the process exits afterwards.
/// </summary>
public static class Screenshot
{
    public static void Schedule(Window window, string path, int delayMs, Action? beforeShot = null)
    {
        var timer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(delayMs) };
        timer.Tick += (_, _) =>
        {
            timer.Stop();
            try
            {
                beforeShot?.Invoke();
                Save(window, path);
            }
            finally
            {
                (Application.Current?.ApplicationLifetime as Avalonia.Controls.ApplicationLifetimes.IClassicDesktopStyleApplicationLifetime)?.Shutdown();
            }
        };
        timer.Start();
    }

    public static void Save(Visual visual, string path)
    {
        var size = visual.Bounds.Size;
        var scale = (visual as TopLevel)?.RenderScaling ?? 1.0;
        var px = new PixelSize(Math.Max(1, (int)Math.Ceiling(size.Width * scale)), Math.Max(1, (int)Math.Ceiling(size.Height * scale)));
        using var bmp = new RenderTargetBitmap(px, new Vector(96 * scale, 96 * scale));
        bmp.Render(visual);
        Directory.CreateDirectory(Path.GetDirectoryName(Path.GetFullPath(path))!);
        bmp.Save(path);
    }

    public static string? Arg(string[] args, string name)
    {
        var prefix = "--" + name + "=";
        foreach (var a in args)
            if (a.StartsWith(prefix, StringComparison.Ordinal)) return a[prefix.Length..];
        return null;
    }
}
