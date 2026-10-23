using Avalonia;
using Avalonia.Controls;
using Avalonia.Interactivity;
using AfviTools.Localization;
using AfviTools.Services;
using Classic.Avalonia.Theme;
using Classic.CommonControls.Dialogs;

namespace AfviTools.Views;

public partial class MainWindow : ClassicWindow
{
    public MainWindow()
    {
        InitializeComponent();
        Width = Math.Round(Width * Math.Max(1, UiSettings.CurrentSize / UiSettings.Default));
        Height = Math.Round(Height * Math.Max(1, UiSettings.CurrentSize / UiSettings.Default));
        BuildFontMenu();
        BuildLanguageMenu();
        Loc.Instance.LanguageChanged += BuildFontMenu; // the "(default)" label is built text
    }

    private void BuildFontMenu()
    {
        FontMenu.Items.Clear();
        var current = (UiSettings.Host ?? Application.Current)?.Resources["W2KFontSize"] is double d ? d : UiSettings.Default;
        foreach (var size in UiSettings.Sizes)
        {
            var item = new MenuItem
            {
                Header = size == UiSettings.Default ? Loc.Instance.Format("font_default", size.ToString("0")) : size.ToString("0"),
                ToggleType = MenuItemToggleType.Radio, GroupName = "fontsize", IsChecked = size == current,
            };
            var s = size;
            item.Click += (_, _) => { UiSettings.Apply(s); UiSettings.Save(s); };
            FontMenu.Items.Add(item);
        }
    }

    private void BuildLanguageMenu()
    {
        LanguageMenu.Items.Clear();
        foreach (var lang in Loc.Instance.Languages)
        {
            var item = new MenuItem
            {
                Header = lang.NativeName, // always in its own language so it can be found from any other
                ToggleType = MenuItemToggleType.Radio, GroupName = "language",
                IsChecked = string.Equals(lang.Tag, Loc.Instance.Tag, StringComparison.OrdinalIgnoreCase),
            };
            var tag = lang.Tag;
            item.Click += (_, _) => { UiSettings.ApplyLanguage(tag); UiSettings.SaveLanguage(tag); };
            LanguageMenu.Items.Add(item);
        }
    }

    private void OnExit(object? sender, RoutedEventArgs e) => Close();

    private async void OnAbout(object? sender, RoutedEventArgs e) =>
        await MessageBox.ShowDialog(this,
            Loc.Instance.Format("about_text", (DataContext as ViewModels.MainViewModel)?.Version ?? "dev"),
            Loc.Instance["about_title"], MessageBoxButtons.Ok, MessageBoxIcon.Information);
}
