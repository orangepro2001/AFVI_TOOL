using AfviTools.Localization;
using AfviTools.Models;
using Avalonia.Media;

namespace AfviTools.ViewModels;

/// <summary>One printed line of a plan: a heading or a step, coloured by its meaning.</summary>
public sealed record PlanLine(string Text, IBrush Brush, FontWeight Weight = FontWeight.Normal);

public sealed class PlanView
{
    private static readonly IBrush Normal = Brushes.Black;
    private static readonly IBrush Danger = new SolidColorBrush(Color.Parse("#C00000"));
    private static readonly IBrush Warn = new SolidColorBrush(Color.Parse("#806000"));

    public PlanView(PlanDto plan)
    {
        Title = plan.Title;
        CanRun = plan.Blockers.Count == 0;
        foreach (var b in plan.Blockers) Problems.Add(new PlanLine(Loc.Instance.Format("plan_blocker", b), Danger, FontWeight.Bold));
        foreach (var w in plan.Warnings) Problems.Add(new PlanLine(Loc.Instance.Format("plan_warning", w), Warn));
        foreach (var u in plan.Units)
        {
            Steps.Add(new PlanLine($"[{u.Target.Machine} / {u.Target.Node}]  {u.Model}" +
                                   (u.NewName != u.Model ? $" → {u.NewName}" : ""), Normal, FontWeight.Bold));
            foreach (var s in u.Steps) Steps.Add(new PlanLine("    " + s.Text, s.Danger ? Danger : Normal));
        }
        Summary = plan.Kind == "copy"
            ? Loc.Instance.Format("plan_summary_copy", plan.Files, Human(plan.Bytes))
            : Loc.Instance.Format("plan_summary_rename", plan.Units.Count);
    }

    public string Title { get; }
    public string Summary { get; }
    public bool CanRun { get; }
    public List<PlanLine> Problems { get; } = [];
    public List<PlanLine> Steps { get; } = [];
    public bool HasProblems => Problems.Count > 0;

    public static string Human(long n)
    {
        string[] u = ["B", "KB", "MB", "GB", "TB"];
        double f = n;
        var i = 0;
        while (f >= 1024 && i < u.Length - 1) { f /= 1024; i++; }
        return i == 0 ? $"{n} B" : $"{f:0.0} {u[i]}";
    }
}
