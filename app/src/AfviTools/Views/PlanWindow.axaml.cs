using Avalonia.Interactivity;
using Classic.Avalonia.Theme;

namespace AfviTools.Views;

public partial class PlanWindow : ClassicWindow
{
    public PlanWindow() => InitializeComponent();

    private void OnRun(object? sender, RoutedEventArgs e) => Close(true);
    private void OnCancel(object? sender, RoutedEventArgs e) => Close(false);
}
