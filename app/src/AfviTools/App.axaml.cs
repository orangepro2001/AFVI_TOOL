using Avalonia;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.Markup.Xaml;
using AfviTools.Localization;
using AfviTools.Services;
using AfviTools.ViewModels;
using AfviTools.Views;
using Classic.CommonControls.Dialogs;

namespace AfviTools;

public partial class App : Application
{
    private CoreHost? _core;

    public override void Initialize()
    {
        AvaloniaXamlLoader.Load(this);
        Services.UiSettings.Host = this;
        // resources first: every window's style reads W2KFontSize / W2KFont dynamically
        var args = Environment.GetCommandLineArgs();
        var lang = Services.Screenshot.Arg(args, "lang") ?? Services.UiSettings.LoadLanguage();
        Services.UiSettings.ApplyLanguage(lang);
        var fs = Services.Screenshot.Arg(args, "fontsize");
        Services.UiSettings.Apply(fs != null && double.TryParse(fs, System.Globalization.CultureInfo.InvariantCulture, out var size)
            ? size : Services.UiSettings.Load());
    }

    public override void OnFrameworkInitializationCompleted()
    {
        if (ApplicationLifetime is IClassicDesktopStyleApplicationLifetime desktop)
        {
            var win = new MainWindow();
            desktop.MainWindow = win;
            desktop.Exit += (_, _) => _core?.Dispose();
            win.Opened += async (_, _) => await StartAsync(win, desktop.Args ?? []);
        }
        base.OnFrameworkInitializationCompleted();
    }

    private async Task StartAsync(MainWindow win, string[] args)
    {
        try
        {
            _core = await CoreHost.Start(Screenshot.Arg(args, "config"), args.Contains("--allow-network-write"));
        }
        catch (Exception ex)
        {
            await MessageBox.ShowDialog(win, Loc.Instance.Format("err_core_start", ex.Message), "AFVI TOOLS",
                MessageBoxButtons.Ok, MessageBoxIcon.Error);
            (ApplicationLifetime as IClassicDesktopStyleApplicationLifetime)?.Shutdown(1);
            return;
        }
        var vm = new MainViewModel(_core.Client, new DialogService(win, _core.Client));
        win.DataContext = vm;
        vm.Version = _core.Version;
        win.Title = $"AFVI TOOLS {_core.Version}";
        await vm.InitializeAsync();

        var machine = Screenshot.Arg(args, "machine");
        if (machine != null)
        {
            var m = vm.Machines.FirstOrDefault(x => string.Equals(x.Name, machine, StringComparison.OrdinalIgnoreCase));
            if (m != null) vm.SelectedMachine = m;
        }
        if (Screenshot.Arg(args, "node") is { } node) vm.SelectedNode = node;
        if (Screenshot.Arg(args, "selftest") is { } dir)
        {
            var code = await SelfTest.Run(win, _core.Client, new DialogService(win, _core.Client), dir);
            (ApplicationLifetime as IClassicDesktopStyleApplicationLifetime)?.Shutdown(code);
            return;
        }
        if (Screenshot.Arg(args, "screenshot") is { } shot)
            Screenshot.Schedule(win, shot, int.Parse(Screenshot.Arg(args, "shot-delay") ?? "2500"));
    }
}
