using System.Collections.ObjectModel;
using AfviTools.Localization;
using AfviTools.Models;
using AfviTools.Services;
using CommunityToolkit.Mvvm.ComponentModel;
using CommunityToolkit.Mvvm.Input;

namespace AfviTools.ViewModels;

/// <summary>Machine configuration dialog: list on the left, editor on the right.</summary>
public partial class MachinesViewModel : ObservableObject
{
    private readonly CoreClient _core;
    private readonly IDialogService _dialogs;
    private string? _originalName; // null while editing a new machine

    public MachinesViewModel(CoreClient core, IDialogService dialogs)
    {
        _core = core;
        _dialogs = dialogs;
    }

    public ObservableCollection<MachineDto> Items { get; } = [];
    [ObservableProperty] private MachineDto? _selected;
    [ObservableProperty] private MachineDto _draft = new();
    [ObservableProperty] private string _message = "";
    [ObservableProperty] private bool _isError;

    /// <summary>True once anything was saved, deleted or restored.</summary>
    public bool Changed { get; private set; }

    public bool IsNew => _originalName == null;
    public string EditorTitle => Loc.Instance[IsNew ? "mach_new" : "mach_edit"];

    partial void OnSelectedChanged(MachineDto? value)
    {
        if (value == null) return;
        _originalName = value.Name;
        Draft = value.Clone();
        OnPropertyChanged(nameof(IsNew));
        OnPropertyChanged(nameof(EditorTitle));
        DeleteCommand.NotifyCanExecuteChanged();
    }

    public async Task LoadAsync(string? select = null)
    {
        var cfg = await _core.GetConfig();
        Items.Clear();
        foreach (var m in cfg.Machines) Items.Add(m);
        Selected = Items.FirstOrDefault(m => string.Equals(m.Name, select, StringComparison.OrdinalIgnoreCase))
                   ?? Items.FirstOrDefault();
        if (Selected == null) New();
    }

    private void Report(string text, bool error)
    {
        Message = text;
        IsError = error;
    }

    [RelayCommand]
    private void New()
    {
        Selected = null;
        _originalName = null;
        Draft = new MachineDto { Name = "AFVI " };
        OnPropertyChanged(nameof(IsNew));
        OnPropertyChanged(nameof(EditorTitle));
        DeleteCommand.NotifyCanExecuteChanged();
        Report("", false);
    }

    [RelayCommand]
    private async Task Save()
    {
        var m = Draft.Clone();
        m.Name = m.Name.Trim();
        foreach (var n in new[] { m.Fm1, m.Fm2, m.Bm })
        {
            n.Host = n.Host.Trim();
            n.RepositoryPath = string.IsNullOrWhiteSpace(n.RepositoryPath) ? null : n.RepositoryPath.Trim();
            n.InventoryPath = string.IsNullOrWhiteSpace(n.InventoryPath) ? null : n.InventoryPath.Trim();
        }
        try
        {
            if (_originalName == null) await _core.AddMachine(m);
            else await _core.UpdateMachine(_originalName, m);
            Changed = true;
            await LoadAsync(m.Name);
            Report(Loc.Instance.Format("mach_saved", m.Name), false);
        }
        catch (CoreException ex)
        {
            Report(ex.Message, true);
        }
    }

    private bool CanDelete() => _originalName != null;

    [RelayCommand(CanExecute = nameof(CanDelete))]
    private async Task Delete()
    {
        var name = _originalName;
        if (name == null) return;
        if (!await _dialogs.Confirm(Loc.Instance["dlg_delete_title"], Loc.Instance.Format("dlg_delete_text", name))) return;
        try
        {
            await _core.DeleteMachine(name);
            Changed = true;
            await LoadAsync();
            Report(Loc.Instance.Format("mach_deleted", name), false);
        }
        catch (CoreException ex)
        {
            Report(ex.Message, true);
        }
    }

    [RelayCommand]
    private async Task RestoreDefault()
    {
        try
        {
            await _core.RestoreDefaults();
            Changed = true;
            await LoadAsync("AFVI 13");
            Report(Loc.Instance["mach_restored"], false);
        }
        catch (CoreException ex)
        {
            Report(ex.Message, true);
        }
    }
}
