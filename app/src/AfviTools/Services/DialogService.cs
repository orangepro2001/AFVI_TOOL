using Avalonia.Controls;
using AfviTools.Localization;
using AfviTools.Models;
using AfviTools.ViewModels;
using AfviTools.Views;
using Classic.CommonControls.Dialogs;

namespace AfviTools.Services;

/// <summary>Windows and message boxes, all modal to the main window.</summary>
public sealed class DialogService(Window owner, CoreClient core) : IDialogService
{
    public async Task ShowMessage(string title, string text, bool error = false) =>
        await MessageBox.ShowDialog(owner, text, title, MessageBoxButtons.Ok,
            error ? MessageBoxIcon.Error : MessageBoxIcon.Information);

    public async Task<bool> Confirm(string title, string text) =>
        await MessageBox.ShowDialog(owner, text, title, MessageBoxButtons.YesNo, MessageBoxIcon.Question)
        == MessageBoxResult.Yes;

    public async Task<bool> ConfirmPlan(PlanDto plan)
    {
        var win = new PlanWindow { DataContext = new PlanView(plan) };
        return await win.ShowDialog<bool>(owner);
    }

    public async Task<JobDto> ShowJob(JobDto job)
    {
        var vm = new JobViewModel(core, job);
        var win = new JobWindow { DataContext = vm };
        await win.ShowDialog(owner);
        if (vm.Final.State == "failed")
            await ShowMessage(Loc.Instance["dlg_op_failed"], vm.Final.Error ?? Loc.Instance["err_unknown"], true);
        return vm.Final;
    }

    public async Task<bool> EditMachines()
    {
        var vm = new MachinesViewModel(core, this);
        var win = new MachinesWindow { DataContext = vm };
        await win.ShowDialog(owner);
        return vm.Changed;
    }
}
