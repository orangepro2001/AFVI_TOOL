using AfviTools.Models;

namespace AfviTools.ViewModels;

/// <summary>What the view models need from the windowing layer.</summary>
public interface IDialogService
{
    Task ShowMessage(string title, string text, bool error = false);
    Task<bool> Confirm(string title, string text);
    /// <summary>Shows the plan and returns true when the user chose to run it.</summary>
    Task<bool> ConfirmPlan(PlanDto plan);
    /// <summary>Shows a modal progress window for a started job and returns the final state.</summary>
    Task<JobDto> ShowJob(JobDto job);
    /// <summary>Opens the machine configuration dialog; true when something changed.</summary>
    Task<bool> EditMachines();
}
