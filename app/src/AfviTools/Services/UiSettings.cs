using System.Text.Json;
using Avalonia;
using AfviTools.Localization;

namespace AfviTools.Services;

/// <summary>Per-user look settings (font size), kept in a small json next to the core config.</summary>
public static class UiSettings
{
    public const double Min = 10, Max = 28, Default = 14;
    public static readonly double[] Sizes = [12, 13, 14, 16, 18, 20];

    /// <summary>
    /// The per-user data folder (settings, the unpacked core): %LOCALAPPDATA%\AFVI_TOOLS, $XDG_DATA_HOME/AFVI_TOOLS or
    /// ~/.local/share/AFVI_TOOLS, ~/Library/Application Support/AFVI_TOOLS. Always absolute: GetFolderPath can return ""
    /// when the folder does not exist yet, which would silently put files in the working directory.
    /// </summary>
    public static string DataDirectory()
    {
        string? root = null;
        if (OperatingSystem.IsWindows()) root = Environment.GetEnvironmentVariable("LOCALAPPDATA");
        else if (OperatingSystem.IsMacOS()) root = Combine(Home(), "Library", "Application Support");
        else root = Environment.GetEnvironmentVariable("XDG_DATA_HOME") is { Length: > 0 } x && Path.IsPathRooted(x) ? x : Combine(Home(), ".local", "share");
        if (string.IsNullOrEmpty(root) || !Path.IsPathRooted(root)) root = Path.Combine(Path.GetTempPath(), "AFVI_TOOLS-" + Environment.UserName);
        return Path.Combine(root, "AFVI_TOOLS");
    }

    private static string? Home() => Environment.GetEnvironmentVariable("HOME") is { Length: > 0 } h ? h : Environment.GetFolderPath(Environment.SpecialFolder.UserProfile);
    private static string? Combine(string? a, params string[] rest) => string.IsNullOrEmpty(a) ? null : Path.Combine([a, .. rest]);

    private static string FilePath => Path.Combine(DataDirectory(), "ui.json");

    private static JsonElement? Read()
    {
        try
        {
            using var doc = JsonDocument.Parse(File.ReadAllText(FilePath));
            return doc.RootElement.Clone();
        }
        catch (Exception ex) when (ex is IOException or JsonException or UnauthorizedAccessException)
        {
            return null;
        }
    }

    public static double Load() =>
        Read() is { } r && r.TryGetProperty("fontSize", out var v) && v.ValueKind == JsonValueKind.Number
            ? Clamp(v.GetDouble()) : Default;

    /// <summary>The saved language tag; Korean when nothing was saved.</summary>
    public static string LoadLanguage() =>
        Read() is { } r && r.TryGetProperty("language", out var v) && v.ValueKind == JsonValueKind.String
            ? v.GetString() ?? Loc.DefaultTag : Loc.DefaultTag;

    public static void Save(double size) => Write(size, Loc.Instance.Tag);

    public static void SaveLanguage(string tag) => Write(Load(), tag);

    private static void Write(double size, string language)
    {
        try
        {
            Directory.CreateDirectory(Path.GetDirectoryName(FilePath)!);
            File.WriteAllText(FilePath, $"{{\"fontSize\":{size.ToString(System.Globalization.CultureInfo.InvariantCulture)},\"language\":{JsonSerializer.Serialize(language, PackJsonString.Default.String)}}}");
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException) { /* a preference, never fatal */ }
    }

    /// <summary>Switches the language: texts (bindings), the font family for its script and the core's wording.</summary>
    public static void ApplyLanguage(string? tag)
    {
        Loc.Instance.Set(tag);
        if (App != null)
        {
            var family = new Avalonia.Media.FontFamily(Loc.Instance.Current.FontFamily);
            App.Resources["W2KFont"] = family;
            // the theme's own key; its default starts with the bundled Tahoma look-alike for Latin text
            App.Resources["ContentControlThemeFontFamily"] =
                new Avalonia.Media.FontFamily("fonts:Tahoma#Tahoma, " + Loc.Instance.Current.FontFamily);
        }
    }

    /// <summary>The application whose resources carry the look settings (set by App.Initialize, before Application.Current exists).</summary>
    public static Application? Host { get; set; }

    private static Application? App => Host ?? Application.Current;

    public static double Clamp(double v) => Math.Min(Max, Math.Max(Min, v));

    /// <summary>Raised after the font size changed.</summary>
    public static event Action? FontSizeChanged;

    public static double CurrentSize { get; private set; } = Default;

    public static void Apply(double size)
    {
        var v = Clamp(size);
        CurrentSize = v;
        if (App == null) { FontSizeChanged?.Invoke(); return; }
        App.Resources["W2KFontSize"] = v;
        // the Classic theme sizes its controls from these three resources
        App.Resources["FontSizeNormal"] = v;
        App.Resources["FontSizeSmall"] = Math.Max(Min, v - 1);
        App.Resources["FontSizeLarge"] = v + 2;
        FontSizeChanged?.Invoke();
    }
}

[System.Text.Json.Serialization.JsonSerializable(typeof(string))]
internal partial class PackJsonString : System.Text.Json.Serialization.JsonSerializerContext;
