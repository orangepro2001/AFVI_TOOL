using Avalonia.Interactivity;
using Classic.Avalonia.Theme;
using AfviTools.ViewModels;

namespace AfviTools.Views;

public partial class JobWindow : ClassicWindow
{
    private readonly CancellationTokenSource _cts = new();

    public JobWindow() => InitializeComponent();

    protected override async void OnOpened(EventArgs e)
    {
        base.OnOpened(e);
        if (DataContext is JobViewModel vm)
        {
            try { await vm.Poll(_cts.Token); } catch (OperationCanceledException) { }
        }
    }

    protected override void OnClosing(Avalonia.Controls.WindowClosingEventArgs e)
    {
        // a running job cannot be abandoned by closing the window
        if (DataContext is JobViewModel { Running: true }) e.Cancel = true;
        else _cts.Cancel();
        base.OnClosing(e);
    }

    private void OnClose(object? sender, RoutedEventArgs e) => Close();
}
