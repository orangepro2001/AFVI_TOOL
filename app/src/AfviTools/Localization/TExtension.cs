using Avalonia;
using Avalonia.Markup.Xaml;

namespace AfviTools.Localization;

/// <summary>
/// XAML: Text="{loc:T menu_file}". Re-evaluates when the language changes. It hands Avalonia an observable of the
/// text, so no reflection binding is involved and it is safe under trimming and Native AOT.
/// </summary>
public class TExtension(string key) : MarkupExtension
{
    public string Key { get; set; } = key;

    public override object ProvideValue(IServiceProvider serviceProvider) => Loc.Instance.Observe(Key).ToBinding();
}
