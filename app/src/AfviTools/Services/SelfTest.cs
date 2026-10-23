using Avalonia.Controls;
using AfviTools.Localization;
using AfviTools.Models;
using AfviTools.ViewModels;
using AfviTools.Views;

namespace AfviTools.Services;

/// <summary>
/// --selftest=&lt;dir&gt;: opens every window in every language with sample data, renders each to a PNG and exits
/// 0 (all fine) or 1. It runs the same build the user gets, so it catches what trimming or a bad XAML attribute
/// breaks only at run time.
/// </summary>
public static class SelfTest
{
    public static async Task<int> Run(MainWindow main, CoreClient core, IDialogService dialogs, string dir)
    {
        Directory.CreateDirectory(dir);
        var failures = new List<string>();
        var sample = new PlanDto("p", "copy", "Copy sample: A FM1 → A FM2",
            [new UnitDto(new NodeRef("A", "FM1"), new NodeRef("A", "FM2"), "sample", "sample",
                [new StepDto("delete", "Delete A FM2: PxRepository\\sample", true), new StepDto("copy", "Copy PxRepository\\sample → PxRepository\\sample (2 files, 10 B)", false)], 2, 10)],
            ["warning text"], ["blocker text"], 2, 10);
        var job = new JobDto("j", "Copy sample", "running", null,
            new ProgressDto("phase", "current/file", 1, 2, 5, 10, false, ["log line"], 0.5));

        foreach (var lang in Loc.Instance.Languages)
        {
            UiSettings.ApplyLanguage(lang.Tag);
            await Check(failures, dir, lang.Tag, "main", main);
            await Check(failures, dir, lang.Tag, "plan", () => new PlanWindow { DataContext = new PlanView(sample) });
            await Check(failures, dir, lang.Tag, "job", () => new JobWindow { DataContext = new JobViewModel(core, job) });
            var machines = new MachinesViewModel(core, dialogs);
            await machines.LoadAsync();
            await Check(failures, dir, lang.Tag, "machines", () => new MachinesWindow { DataContext = machines });
        }
        foreach (var f in failures) Console.Error.WriteLine("SELFTEST FAIL: " + f);
        Console.WriteLine(failures.Count == 0 ? "SELFTEST OK" : $"SELFTEST {failures.Count} failure(s)");
        return failures.Count == 0 ? 0 : 1;
    }

    private static Task Check(List<string> failures, string dir, string lang, string name, Window w) => Render(failures, dir, lang, name, w, false);

    private static Task Check(List<string> failures, string dir, string lang, string name, Func<Window> make)
    {
        try { return Render(failures, dir, lang, name, make(), true); }
        catch (Exception ex) { failures.Add($"{lang}/{name}: {ex.GetType().Name}: {ex.Message}"); return Task.CompletedTask; }
    }

    private static async Task Render(List<string> failures, string dir, string lang, string name, Window w, bool close)
    {
        try
        {
            if (close) w.Show();
            await Task.Delay(700); // layout, bindings, first poll
            var path = Path.Combine(dir, $"{name}-{lang}.png");
            Screenshot.Save(w, path);
            if (new FileInfo(path).Length < 3000) failures.Add($"{lang}/{name}: blank render");
        }
        catch (Exception ex) { failures.Add($"{lang}/{name}: {ex.GetType().Name}: {ex.Message}"); }
        finally { if (close) { try { w.DataContext = null; w.Close(); } catch (Exception) { } } }
    }
}
