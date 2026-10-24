using System.ComponentModel;
using System.Globalization;
using System.Reflection;
using System.Diagnostics.CodeAnalysis;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace AfviTools.Localization;

/// <summary>One selectable language.</summary>
public sealed record LangInfo(string Tag, string NativeName, string FontFamily);

/// <summary>
/// The language packs of the GUI. Packs are flat key -> text json files:
/// the four built-in ones are embedded (Assets/lang), and every *.json in a
/// "lang" folder next to the program adds a language or overrides keys of one
/// (the file name is the tag, the optional "_name" entry the menu text).
/// Default language: Korean.
/// </summary>
[JsonSerializable(typeof(Dictionary<string, string>))]
internal partial class PackJson : JsonSerializerContext;

[DynamicallyAccessedMembers(DynamicallyAccessedMemberTypes.All)] // reflection bindings read the indexer
public sealed class Loc : INotifyPropertyChanged
{
    public const string DefaultTag = "ko";

    public static Loc Instance { get; } = new();

    private readonly Dictionary<string, Dictionary<string, string>> _packs = new(StringComparer.OrdinalIgnoreCase);
    private readonly List<LangInfo> _langs = [];
    private Dictionary<string, string> _current = new();
    private Dictionary<string, string> _fallback = new();

    public event PropertyChangedEventHandler? PropertyChanged;
    public event Action? LanguageChanged;

    public string Tag { get; private set; } = DefaultTag;
    public IReadOnlyList<LangInfo> Languages => _langs;

    private Loc()
    {
        // Tahoma first (Windows look), then the CJK font that has the right glyph forms for the language;
        // the Linux dev machine falls back to Noto Sans CJK.
        _langs.Add(new("ko", "한국어", "Tahoma, Malgun Gothic, Noto Sans CJK KR, Noto Sans CJK SC, DejaVu Sans"));
        _langs.Add(new("zh-Hans", "简体中文", "Tahoma, Microsoft YaHei, Noto Sans CJK SC, DejaVu Sans"));
        _langs.Add(new("zh-Hant", "繁體中文", "Tahoma, Microsoft JhengHei, Noto Sans CJK TC, DejaVu Sans"));
        _langs.Add(new("en", "English", "Tahoma, Segoe UI, Noto Sans CJK SC, DejaVu Sans"));
        foreach (var l in _langs) _packs[l.Tag] = ReadEmbedded(l.Tag);
        LoadExternal(Path.Combine(AppContext.BaseDirectory, "lang"));
        _fallback = _packs["en"];
        _current = _packs[DefaultTag];
    }

    private static Dictionary<string, string> ReadEmbedded(string tag)
    {
        var asm = typeof(Loc).Assembly;
        var name = $"lang.{tag}.json";
        using var s = asm.GetManifestResourceStream(name)
                      ?? throw new InvalidOperationException("missing embedded language pack " + name);
        return JsonSerializer.Deserialize(s, PackJson.Default.DictionaryStringString)
               ?? throw new InvalidOperationException("empty language pack " + name);
    }

    /// <summary>Adds or overrides packs from a folder; broken files are skipped.</summary>
    public void LoadExternal(string dir)
    {
        if (!Directory.Exists(dir)) return;
        foreach (var file in Directory.EnumerateFiles(dir, "*.json"))
        {
            try
            {
                var tag = Path.GetFileNameWithoutExtension(file);
                var pack = JsonSerializer.Deserialize(File.ReadAllText(file), PackJson.Default.DictionaryStringString);
                if (pack == null || tag.Length == 0) continue;
                if (!_packs.TryGetValue(tag, out var existing))
                {
                    _packs[tag] = existing = new Dictionary<string, string>();
                    var font = _langs[0].FontFamily;
                    _langs.Add(new(tag, pack.GetValueOrDefault("_name", tag), font));
                }
                foreach (var kv in pack) existing[kv.Key] = kv.Value;
            }
            catch (Exception ex) when (ex is IOException or JsonException or UnauthorizedAccessException)
            {
                System.Diagnostics.Trace.WriteLine($"language pack {file} skipped: {ex.Message}");
            }
        }
    }

    public LangInfo Current => _langs.First(l => string.Equals(l.Tag, Tag, StringComparison.OrdinalIgnoreCase));

    /// <summary>Maps "ko-KR", "zh-CN", "zh-TW", "en-US" ... to a known tag; unknown gives the default.</summary>
    public string Resolve(string? tag)
    {
        if (string.IsNullOrWhiteSpace(tag)) return DefaultTag;
        var exact = _langs.FirstOrDefault(l => string.Equals(l.Tag, tag, StringComparison.OrdinalIgnoreCase));
        if (exact != null) return exact.Tag;
        var t = tag.ToLowerInvariant();
        if (t.StartsWith("ko")) return "ko";
        if (t.StartsWith("en")) return "en";
        if (t.StartsWith("zh"))
            return t.Contains("hant") || t.EndsWith("-tw") || t.EndsWith("-hk") || t.EndsWith("-mo") ? "zh-Hant" : "zh-Hans";
        return DefaultTag;
    }

    public void Set(string? tag)
    {
        var resolved = Resolve(tag);
        if (string.Equals(resolved, Tag, StringComparison.OrdinalIgnoreCase) && ReferenceEquals(_current, _packs[Tag])) return;
        Tag = resolved;
        _current = _packs[resolved];
        // Avalonia's indexer binding looks the changed property up by name on the type: that is "Item"
        // for a C# indexer ("Item[]" is the WPF/INCC convention); raise both.
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs("Item"));
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs("Item[]"));
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(nameof(Tag)));
        LanguageChanged?.Invoke();
    }

    /// <summary>The text of a key; falls back to English, then to the key itself (a gap shows, never crashes).</summary>
    public string this[string key] =>
        _current.TryGetValue(key, out var s) ? s : _fallback.TryGetValue(key, out s) ? s : key;

    public string Format(string key, params object?[] args) =>
        string.Format(CultureInfo.InvariantCulture, this[key], args);

    /// <summary>
    /// The text of a key as an observable: the current text on subscribe, then again after every language change.
    /// Turned into a binding with <c>ToBinding()</c> this needs no reflection (Native AOT safe).
    /// </summary>
    public IObservable<string> Observe(string key) => new KeyObservable(this, key);

    private sealed class KeyObservable(Loc loc, string key) : IObservable<string>
    {
        public IDisposable Subscribe(IObserver<string> observer)
        {
            void Push() => observer.OnNext(loc[key]);
            loc.LanguageChanged += Push;
            Push();
            return new Unsub(() => loc.LanguageChanged -= Push);
        }

        private sealed class Unsub(Action undo) : IDisposable
        {
            private Action? _undo = undo;
            public void Dispose() => Interlocked.Exchange(ref _undo, null)?.Invoke();
        }
    }

    /// <summary>Every key of the English pack (tests check that all packs have them).</summary>
    public IEnumerable<string> Keys => _packs["en"].Keys.Where(k => !k.StartsWith('_'));

    public IReadOnlyDictionary<string, string> Pack(string tag) => _packs[tag];
}
