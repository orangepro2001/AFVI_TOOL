using System.Collections.ObjectModel;
using AfviTools.Localization;
using AfviTools.Models;
using AfviTools.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace AfviTools.ViewModels;

/// <summary>Polls a running job and exposes its progress.</summary>
public partial class JobViewModel : ObservableObject
{
    private readonly CoreClient _core;
    private readonly string _id;

    public JobViewModel(CoreClient core, JobDto job)
    {
        _core = core;
        _id = job.Id;
        Title = job.Title;
        Apply(job);
        Final = job;
    }

    public string Title { get; }
    [ObservableProperty] private string _windowTitle = Loc.Instance["job_running"];
    public ObservableCollection<string> Log { get; } = [];
    [ObservableProperty] private string _phase = "";
    [ObservableProperty] private string _current = "";
    [ObservableProperty] private string _counts = "";
    [ObservableProperty] private double _percent;
    [ObservableProperty] private bool _running = true;
    [ObservableProperty] private bool _canCancel = true;
    [ObservableProperty] private string _result = "";
    [ObservableProperty] private bool _failed;
    public JobDto Final { get; private set; }

    private void Apply(JobDto j)
    {
        Final = j;
        var p = j.Progress;
        Phase = p.Phase;
        Current = p.Current;
        Counts = Loc.Instance.Format("job_counts", p.FilesDone, p.FilesTotal, PlanView.Human(p.BytesDone), PlanView.Human(p.BytesTotal), p.Seconds.ToString("0.0", System.Globalization.CultureInfo.InvariantCulture));
        Percent = p.BytesTotal > 0 ? Math.Min(100, 100.0 * p.BytesDone / p.BytesTotal)
            : p.FilesTotal > 0 ? Math.Min(100, 100.0 * p.FilesDone / p.FilesTotal) : 0;
        CanCancel = !p.Committed && j.State == "running";
        for (var i = Log.Count; i < p.Log.Count; i++) Log.Add(p.Log[i]);
        if (j.State != "running")
        {
            Running = false;
            CanCancel = false;
            Failed = j.State == "failed";
            if (j.State == "done") { Percent = 100; Result = Loc.Instance["job_done"]; WindowTitle = Loc.Instance["job_done"]; }
            else { Result = j.Error ?? j.State; WindowTitle = j.State == "canceled" ? Loc.Instance["job_canceled"] : Loc.Instance["job_failed"]; }
        }
    }

    /// <summary>Polls until the job leaves the running state.</summary>
    public async Task Poll(CancellationToken ct)
    {
        while (Running && !ct.IsCancellationRequested)
        {
            await Task.Delay(150, ct);
            try { Apply(await _core.GetJob(_id, ct)); }
            catch (CoreException ex) { Failed = true; Running = false; Result = ex.Message; }
        }
    }

    [RelayCommand]
    private async Task Cancel()
    {
        CanCancel = false;
        try { await _core.CancelJob(_id); }
        catch (CoreException) { }
    }
}
