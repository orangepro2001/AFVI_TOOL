using System.Collections.ObjectModel;
using AfviTools.Localization;
using AfviTools.Models;
using AfviTools.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace AfviTools.ViewModels;

public partial class MainViewModel : ObservableObject
{
    public static readonly string[] NodeKinds = ["FM1", "FM2", "BM"];

    private readonly CoreClient _core;
    private readonly IDialogService _dialogs;
    private static Loc L => Loc.Instance;
    private List<ModelRowDto> _dtos = [];
    private List<ModelRow> _all = [];
    private int _loadSeq;

    public MainViewModel(CoreClient core, IDialogService dialogs)
    {
        _core = core;
        _dialogs = dialogs;
        L.LanguageChanged += OnLanguageChanged;
    }

    public string Version { get; set; } = "";
    private string _statusKey = "st_ready";
    private object?[] _statusArgs = [];

    /// <summary>Sets the status line from a language key; it is re-worded when the language changes.</summary>
    private void SetStatus(string key, params object?[] args)
    {
        _statusKey = key;
        _statusArgs = args;
        StatusText = L.Format(key, args);
    }

    private bool _lockOpen;
    private bool _lockReadOnly = true;

    private void OnLanguageChanged()
    {
        // rows and lock text are built strings: rebuild them in the new language
        _all = _dtos.Select(d => new ModelRow(d)).ToList();
        ApplyFilter();
        UpdateLockText();
        StatusText = L.Format(_statusKey, _statusArgs);
        OnPropertyChanged(nameof(SourceLabel));
    }

    private void UpdateLockText() =>
        LockText = _lockReadOnly ? (_lockOpen ? L["lock_open"] : L["lock_locked"]) : L["lock_writable"];

    // ---- source ---------------------------------------------------------------
    public ObservableCollection<MachineDto> Machines { get; } = [];
    public string[] Nodes => NodeKinds;
    public ObservableCollection<ModelRow> Models { get; } = [];

    [ObservableProperty] private MachineDto? _selectedMachine;
    [ObservableProperty] private string _selectedNode = "FM1";
    [ObservableProperty] private string _searchText = "";
    [ObservableProperty] private ModelRow? _selectedModel;

    // ---- status bar -----------------------------------------------------------
    [ObservableProperty] private string _statusText = Loc.Instance["st_ready"];
    [ObservableProperty] private string _pathText = "";
    [ObservableProperty] private string _countText = "";
    [ObservableProperty] private string _lockText = "";
    [ObservableProperty] private bool _isBusy;

    // ---- copy -----------------------------------------------------------------
    public ObservableCollection<NodeRef> SingleTargets { get; } = [];
    public ObservableCollection<string> GroupTargets { get; } = [];
    [ObservableProperty] private bool _copyAsGroup;
    [ObservableProperty] private NodeRef? _selectedSingleTarget;
    [ObservableProperty] private string? _selectedGroupTarget;
    [ObservableProperty] private string _copyNewName = "";
    public bool CanGroupCopy => GroupTargets.Count > 0;
    public bool CanSingleCopy => SingleTargets.Count > 0;

    // ---- rename ---------------------------------------------------------------
    [ObservableProperty] private bool _renameAsGroup;
    [ObservableProperty] private string _renameNewName = "";

    public string SourceTitle => SelectedMachine == null ? "" : $"{SelectedMachine.Name} / {SelectedNode}";
    public string SourceLabel => L.Format("lbl_source_is", SourceTitle);

    partial void OnSelectedMachineChanged(MachineDto? value) { OnPropertyChanged(nameof(SourceTitle)); OnPropertyChanged(nameof(SourceLabel)); _ = ReloadAsync(); }
    partial void OnSelectedNodeChanged(string value) { OnPropertyChanged(nameof(SourceTitle)); OnPropertyChanged(nameof(SourceLabel)); _ = ReloadAsync(); }
    partial void OnSearchTextChanged(string value) => ApplyFilter();
    partial void OnSelectedModelChanged(ModelRow? value)
    {
        CopyCommand.NotifyCanExecuteChanged();
        RenameCommand.NotifyCanExecuteChanged();
    }
    partial void OnCopyAsGroupChanged(bool value) => CopyCommand.NotifyCanExecuteChanged();
    partial void OnSelectedSingleTargetChanged(NodeRef? value) => CopyCommand.NotifyCanExecuteChanged();
    partial void OnSelectedGroupTargetChanged(string? value) => CopyCommand.NotifyCanExecuteChanged();
    partial void OnIsBusyChanged(bool value)
    {
        CopyCommand.NotifyCanExecuteChanged();
        RenameCommand.NotifyCanExecuteChanged();
    }

    /// <summary>Loads the configuration; called once after the core is up.</summary>
    public async Task InitializeAsync()
    {
        try
        {
            var health = await _core.Health();
            _lockOpen = health.AllowNetworkWrite;
            UpdateLockText();
            await LoadMachinesAsync(null);
        }
        catch (Exception ex)
        {
            SetStatus("st_init_fail", ex.Message);
            await _dialogs.ShowMessage("AFVI TOOLS", L.Format("msg_cfg_fail", ex.Message), true);
        }
    }

    public async Task LoadMachinesAsync(string? keep)
    {
        var cfg = await _core.GetConfig();
        Machines.Clear();
        foreach (var m in cfg.Machines) Machines.Add(m);
        SelectedMachine = Machines.FirstOrDefault(m => string.Equals(m.Name, keep, StringComparison.OrdinalIgnoreCase))
                          ?? Machines.FirstOrDefault();
        if (SelectedMachine == null) { Models.Clear(); _all = []; CountText = L.Format("count_models", 0); }
    }

    [RelayCommand]
    private Task Refresh() => ReloadAsync();

    public async Task ReloadAsync(bool keepStatus = false)
    {
        var machine = SelectedMachine;
        var node = SelectedNode;
        if (machine == null) return;
        var seq = ++_loadSeq;
        if (!keepStatus) SetStatus("st_reading", machine.Name, node);
        try
        {
            var listTask = _core.ListModels(machine.Name, node);
            var targetsTask = _core.Targets(machine.Name, node);
            var list = await listTask;
            var targets = await targetsTask;
            if (seq != _loadSeq) return; // a newer load is under way
            _dtos = list.Models;
            _all = _dtos.Select(m => new ModelRow(m)).ToList();
            PathText = list.Repository;
            _lockReadOnly = list.ReadOnly;
            UpdateLockText();
            SingleTargets.Clear();
            foreach (var t in targets.Single) SingleTargets.Add(t);
            GroupTargets.Clear();
            foreach (var g in targets.Group) GroupTargets.Add(g);
            SelectedSingleTarget = SingleTargets.FirstOrDefault(t => t.Machine == machine.Name && t.Node != node)
                                   ?? SingleTargets.FirstOrDefault();
            SelectedGroupTarget = GroupTargets.FirstOrDefault();
            if (!CanGroupCopy) CopyAsGroup = false;
            OnPropertyChanged(nameof(CanGroupCopy));
            OnPropertyChanged(nameof(CanSingleCopy));
            ApplyFilter();
            if (!keepStatus) SetStatus("st_read_ok", machine.Name, node, _all.Count);
        }
        catch (Exception ex)
        {
            if (seq != _loadSeq) return;
            _all = [];
            _dtos = [];
            Models.Clear();
            CountText = "";
            SetStatus("st_read_fail", machine.Name, node, ex.Message);
        }
    }

    private void ApplyFilter()
    {
        var q = SearchText.Trim();
        var keep = SelectedModel?.Name;
        Models.Clear();
        foreach (var m in _all)
            if (q.Length == 0 || m.Name.Contains(q, StringComparison.OrdinalIgnoreCase))
                Models.Add(m);
        SelectedModel = Models.FirstOrDefault(m => string.Equals(m.Name, keep, StringComparison.OrdinalIgnoreCase));
        CountText = q.Length == 0 ? L.Format("count_models", _all.Count) : L.Format("count_filtered", Models.Count, _all.Count);
    }

    private bool HasModel => SelectedModel != null && !IsBusy;

    private bool CanCopy() => HasModel && (CopyAsGroup ? SelectedGroupTarget != null : SelectedSingleTarget != null);

    [RelayCommand(CanExecute = nameof(CanCopy))]
    private async Task Copy()
    {
        var machine = SelectedMachine;
        var model = SelectedModel;
        if (machine == null || model == null) return;
        var src = new NodeRef(machine.Name, SelectedNode);
        var newName = CopyNewName.Trim();
        CopyRequestDto req = CopyAsGroup
            ? new("group", src, new NodeRef(SelectedGroupTarget!, SelectedNode), model.Name, newName)
            : new("single", src, SelectedSingleTarget!, model.Name, newName);
        await RunPlan(() => _core.PlanCopy(req));
    }

    private bool CanRename() => HasModel;

    [RelayCommand(CanExecute = nameof(CanRename))]
    private async Task Rename()
    {
        var machine = SelectedMachine;
        var model = SelectedModel;
        if (machine == null || model == null) return;
        var req = new RenameRequestDto(RenameAsGroup ? "group" : "single", new NodeRef(machine.Name, SelectedNode),
            model.Name, RenameNewName.Trim());
        await RunPlan(() => _core.PlanRename(req));
    }

    /// <summary>plan -> confirm -> run -> refresh. Nothing changes before the user confirms.</summary>
    private async Task RunPlan(Func<Task<PlanDto>> plan)
    {
        IsBusy = true;
        try
        {
            SetStatus("st_planning");
            PlanDto p;
            try { p = await plan(); }
            catch (CoreException ex)
            {
                SetStatus("st_cannot", ex.Message);
                await _dialogs.ShowMessage(L["dlg_cannot"], ex.Message, true);
                return;
            }
            if (!await _dialogs.ConfirmPlan(p))
            {
                SetStatus("st_canceled");
                return;
            }
            JobDto job;
            try { job = await _core.StartJob(p.Id); }
            catch (CoreException ex)
            {
                await _dialogs.ShowMessage(L["dlg_cannot_start"], ex.Message, true);
                return;
            }
            StatusText = p.Title + " ...";
            var done = await _dialogs.ShowJob(job);
            if (done.State == "done") SetStatus("st_done", p.Title);
            else if (done.State == "canceled") SetStatus("st_job_canceled", p.Title);
            else SetStatus("st_failed", done.Error ?? p.Title);
            await ReloadAsync(keepStatus: true);
        }
        catch (Exception ex)
        {
            SetStatus("st_error", ex.Message);
            await _dialogs.ShowMessage(L["dlg_error"], ex.Message, true);
        }
        finally
        {
            IsBusy = false;
        }
    }

    [RelayCommand]
    private async Task EditMachines()
    {
        var keep = SelectedMachine?.Name;
        if (await _dialogs.EditMachines())
            await LoadMachinesAsync(keep);
    }

    [RelayCommand]
    private async Task ExportExcel()
    {
        try
        {
            await _core.ExportExcel();
        }
        catch (CoreException ex) when (ex.Status == 501)
        {
            await _dialogs.ShowMessage(L["tab_export"], ex.Message + "\n" + L["export_note"]);
        }
        catch (Exception ex)
        {
            await _dialogs.ShowMessage(L["tab_export"], ex.Message, true);
        }
    }
}
