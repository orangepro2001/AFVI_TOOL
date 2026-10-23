using Avalonia.Interactivity;
using Classic.Avalonia.Theme;
using AfviTools.ViewModels;

namespace AfviTools.Views;

public partial class MachinesWindow : ClassicWindow
{
    public MachinesWindow() => InitializeComponent();

    protected override async void OnOpened(EventArgs e)
    {
        base.OnOpened(e);
        if (DataContext is MachinesViewModel vm) await vm.LoadAsync();
    }

    private void OnClose(object? sender, RoutedEventArgs e) => Close();
}
