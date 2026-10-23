using Avalonia.Controls;
using Avalonia.Headless;
using Avalonia.Headless.XUnit;
using Avalonia.Threading;
using AfviTools.Models;
using AfviTools.Services;
using AfviTools.ViewModels;
using AfviTools.Views;

namespace AfviTools.Tests;

/// <summary>
/// Renders the real windows (Skia, headless) to PNG files under shots/ so the
/// Windows 2000 look can be reviewed, and checks they are not blank.
/// </summary>
public class WindowRenderTests
{
    private static string ShotDir
    {
        get
        {
            var d = AppContext.BaseDirectory;
            while (d != null && !File.Exists(Path.Combine(d, "AFVI_TOOLS.md")) && !Directory.Exists(Path.Combine(d, ".git")))
                d = Path.GetDirectoryName(d);
            var dir = Path.Combine(d ?? Path.GetTempPath(), "shots");
            Directory.CreateDirectory(dir);
            return dir;
        }
    }

    private static void Capture(Window w, string name)
    {
        w.Show();
        Dispatcher.UIThread.RunJobs();
        var frame = w.CaptureRenderedFrame() ?? throw new InvalidOperationException("no frame");
        var path = Path.Combine(ShotDir, name + ".png");
        frame.Save(path);
        Assert.True(new FileInfo(path).Length > 3000, name + " rendered blank");
    }

    [AvaloniaFact]
    public async Task MainWindowWithLocalData()
    {
        await using var h = await Harness.Create();
        h.Dialogs.Client = h.Client;
        foreach (var n in new[] { "FM1", "FM2", "BM" })
        {
            h.AddModel("T1", n, "24fbo060-19", n);
            h.AddModel("T1", n, "7M99-535", n);
        }
        h.AddModel("T1", "FM1", "21csp043-09", "x");
        var vm = h.NewViewModel();
        await vm.InitializeAsync();
        await vm.ReloadAsync();
        vm.SelectedModel = vm.Models[0];
        var win = new MainWindow { DataContext = vm, Width = 1100, Height = 720 };
        Capture(win, "ui-main-local");
        win.Close();
    }

    [AvaloniaFact]
    public async Task MainWindowInEveryLanguage()
    {
        await using var h = await Harness.Create();
        h.Dialogs.Client = h.Client;
        foreach (var n in new[] { "FM1", "FM2", "BM" }) h.AddModel("T1", n, "24fbo060-19", n);
        h.AddModel("T1", "FM1", "7M99-535", "x");
        var vm = h.NewViewModel();
        await vm.InitializeAsync();
        await vm.ReloadAsync();
        vm.SelectedModel = vm.Models[0];
        var win = new MainWindow { DataContext = vm, Width = 1100, Height = 720 };
        try
        {
            foreach (var tag in new[] { "ko", "zh-Hans", "zh-Hant", "en" })
            {
                UiSettings.ApplyLanguage(tag);
                Dispatcher.UIThread.RunJobs();
                Capture(win, "ui-main-" + tag);
            }
        }
        finally { win.Close(); UiSettings.ApplyLanguage("en"); }
    }

    [AvaloniaFact]
    public async Task PlanAndJobWindows()
    {
        await using var h = await Harness.Create();
        h.Dialogs.Client = h.Client;
        foreach (var n in new[] { "FM1", "FM2", "BM" }) h.AddModel("T1", n, "g1", n);
        h.AddModel("T2", "FM1", "g1", "old");
        var plan = await h.Client.PlanCopy(new CopyRequestDto("group", new NodeRef("T1", "FM1"), new NodeRef("T2", "FM1"), "g1", "g1-new"));
        var win = new PlanWindow { DataContext = new PlanView(plan), Width = 760, Height = 520 };
        Capture(win, "ui-plan");
        win.Close();

        var job = await h.Client.StartJob(plan.Id);
        var jvm = new JobViewModel(h.Client, job);
        using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(20));
        await jvm.Poll(cts.Token);
        var jwin = new JobWindow { DataContext = jvm, Width = 640, Height = 420 };
        Capture(jwin, "ui-job-done");
        jwin.Close();
        Assert.Equal("done", jvm.Final.State);
    }

    [AvaloniaFact]
    public async Task MachinesWindow()
    {
        await using var h = await Harness.Create();
        await h.Client.RestoreDefaults();
        var vm = new MachinesViewModel(h.Client, h.Dialogs);
        await vm.LoadAsync("AFVI 13");
        var win = new MachinesWindow { DataContext = vm, Width = 820, Height = 470 };
        Capture(win, "ui-machines");
        win.Close();
    }

    [AvaloniaFact]
    public void BlockedPlanShowsRedReason()
    {
        // a synthetic plan: no network is touched by the tests
        var tgt = new NodeRef("AFVI 13", "FM2");
        var plan = new PlanDto("p1", "copy", "复制 24fbo060-19: AFVI 13 FM1 → AFVI 13 FM2",
            [new UnitDto(new NodeRef("AFVI 13", "FM1"), tgt, "24fbo060-19", "24fbo060-19",
                [new StepDto("delete", "删除 AFVI 13 FM2: PxRepository\\24fbo060-19", true),
                 new StepDto("copy", "复制 PxRepository\\24fbo060-19 → PxRepository\\24fbo060-19 (184 个文件, 805.2 MB)", false)], 184, 844000000)],
            ["AFVI 13 FM2 已存在型号 24fbo060-19,将先删除再复制"],
            ["AFVI 13 FM2 当前为只读(开发阶段网络路径不允许写入),无法作为目标"], 184, 844000000);
        var vm = new PlanView(plan);
        Assert.False(vm.CanRun);
        var win = new PlanWindow { DataContext = vm, Width = 760, Height = 420 };
        Capture(win, "ui-plan-blocked");
        win.Close();
    }
}
