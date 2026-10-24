using System.ComponentModel;
using Avalonia.Collections;
using Avalonia.Controls;
using Avalonia.Controls.ApplicationLifetimes;
using Avalonia.LogicalTree;
using Avalonia.VisualTree;
using AfviTools.Localization;
using Controls = AfviTools.Controls;
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
            await Check(failures, dir, lang.Tag, "main", main, w => Expect(w, ("tab_copy", w.GetLogicalDescendants().OfType<TabItem>().ElementAtOrDefault(0)?.Header), ("tab_export", w.GetLogicalDescendants().OfType<TabItem>().ElementAtOrDefault(1)?.Header), ("menu_file", w.GetLogicalDescendants().OfType<MenuItem>().FirstOrDefault()?.Header), ("lbl_machine", w.GetLogicalDescendants().OfType<TextBlock>().FirstOrDefault(t => t.Text == Loc.Instance["lbl_machine"])?.Text)));
            await Check(failures, dir, lang.Tag, "plan", () => new PlanWindow { DataContext = new PlanView(sample) }, w => Expect(w, ("plan_title", w.Title), ("btn_cancel", w.GetLogicalDescendants().OfType<Button>().LastOrDefault()?.Content)));
            await Check(failures, dir, lang.Tag, "job", () => new JobWindow { DataContext = new JobViewModel(core, job) }, w => Expect(w, ("btn_close", w.GetLogicalDescendants().OfType<Button>().LastOrDefault()?.Content), ("gb_log", w.GetLogicalDescendants().OfType<Controls.GroupBox>().FirstOrDefault()?.Header)));
            var machines = new MachinesViewModel(core, dialogs);
            await machines.LoadAsync();
            await Check(failures, dir, lang.Tag, "machines", () => new MachinesWindow { DataContext = machines }, w => Expect(w, ("mach_title", w.Title), ("btn_save", w.GetLogicalDescendants().OfType<Button>().LastOrDefault(b => b.Content is string c && c == Loc.Instance["btn_save"])?.Content)));
        }
        // DataGrid header sorting goes through DataGridSortDescription.FromPath (property reflection)
        try
        {
            var grid = main.GetVisualDescendants().OfType<DataGrid>().FirstOrDefault();
            var view = grid?.CollectionView;
            if (grid == null || view == null || view.Cast<object>().Count() < 2) failures.Add("sort: no grid or fewer than two rows");
            else
            {
                view.SortDescriptions.Clear();
                view.SortDescriptions.Add(DataGridSortDescription.FromPath("Name", ListSortDirection.Descending));
                var first = (view.Cast<ViewModels.ModelRow>().First()).Name;
                var expect = view.Cast<ViewModels.ModelRow>().Select(r => r.Name).OrderByDescending(n => n, StringComparer.CurrentCulture).First();
                if (first != expect) failures.Add($"sort: first row after descending sort is {first}, expected {expect}");
                view.SortDescriptions.Clear();
            }
        }
        catch (Exception ex) { failures.Add($"sort: {ex.GetType().Name}: {ex.Message}"); }

        // the Classic message box (Classic.CommonControls templates use reflection bindings of their own)
        foreach (var lang in Loc.Instance.Languages)
        {
            UiSettings.ApplyLanguage(lang.Tag);
            var text = "Message body " + lang.Tag;
            _ = dialogs.ShowMessage("Title " + lang.Tag, text);
            await Task.Delay(900);
            var box = (Avalonia.Application.Current?.ApplicationLifetime as IClassicDesktopStyleApplicationLifetime)?.Windows.LastOrDefault(x => x != main);
            if (box == null) { failures.Add($"{lang.Tag}/messagebox: not shown"); continue; }
            var texts = box.GetVisualDescendants().OfType<TextBlock>().Select(t => t.Text).ToList();
            var buttons = box.GetVisualDescendants().OfType<Button>().Count();
            if (!texts.Any(t => t != null && t.Contains(text))) failures.Add($"{lang.Tag}/messagebox: body text missing (texts: {string.Join("|", texts)})");
            if (buttons == 0) failures.Add($"{lang.Tag}/messagebox: no button");
            try { Screenshot.Save(box, Path.Combine(dir, $"messagebox-{lang.Tag}.png")); } catch (Exception ex) { failures.Add($"{lang.Tag}/messagebox: {ex.Message}"); }
            box.Close();
            await Task.Delay(200);
        }
        foreach (var f in failures) Console.Error.WriteLine("SELFTEST FAIL: " + f);
        Console.WriteLine(failures.Count == 0 ? "SELFTEST OK" : $"SELFTEST {failures.Count} failure(s)");
        return failures.Count == 0 ? 0 : 1;
    }

    private static Task Check(List<string> failures, string dir, string lang, string name, Window w, Func<Window, string?> verify) =>
        Render(failures, dir, lang, name, w, false, verify);

    private static Task Check(List<string> failures, string dir, string lang, string name, Func<Window> make, Func<Window, string?> verify)
    {
        try { return Render(failures, dir, lang, name, make(), true, verify); }
        catch (Exception ex) { failures.Add($"{lang}/{name}: {ex.GetType().Name}: {ex.Message}"); return Task.CompletedTask; }
    }

    /// <summary>Each pair is (language key, the value found in the window); a binding that did not resolve (blank, or the view model type name) fails.</summary>
    private static string? Expect(Window w, params (string Key, object? Actual)[] pairs)
    {
        foreach (var (key, actual) in pairs)
            if (!string.Equals(actual as string, Loc.Instance[key], StringComparison.Ordinal))
                return $"'{key}' shows {(actual is null ? "nothing" : $"\"{actual}\"")}, expected \"{Loc.Instance[key]}\"";
        return null;
    }

    private static async Task Render(List<string> failures, string dir, string lang, string name, Window w, bool close, Func<Window, string?> verify)
    {
        try
        {
            if (close) w.Show();
            await Task.Delay(700); // layout, bindings, first poll
            if (verify(w) is { } problem) failures.Add($"{lang}/{name}: {problem}");
            var path = Path.Combine(dir, $"{name}-{lang}.png");
            Screenshot.Save(w, path);
            if (new FileInfo(path).Length < 3000) failures.Add($"{lang}/{name}: blank render");
        }
        catch (Exception ex) { failures.Add($"{lang}/{name}: {ex.GetType().Name}: {ex.Message}"); }
        finally { if (close) { try { w.DataContext = null; w.Close(); } catch (Exception) { } } }
    }
}
