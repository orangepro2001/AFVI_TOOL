using System.Diagnostics.CodeAnalysis;
using Avalonia.Data;
using Avalonia.Markup.Xaml;

namespace AfviTools.Localization;

/// <summary>XAML: Text="{loc:T menu_file}". Re-evaluates when the language changes.</summary>
public class TExtension(string key) : MarkupExtension
{
    public string Key { get; set; } = key;

    // a reflection binding by design (the key is a string indexer): keep the indexer through trimming
    [UnconditionalSuppressMessage("Trimming", "IL2026", Justification = "Loc is annotated to keep all members")]
    public override object ProvideValue(IServiceProvider serviceProvider) =>
        new Binding($"[{Key}]") { Source = Loc.Instance, Mode = BindingMode.OneWay };
}
