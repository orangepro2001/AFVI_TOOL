using AfviTools.Models;
using AfviTools.ViewModels;

namespace AfviTools.Tests;

public class ViewModelFlowTests
{
    private static async Task<(Harness h, MainViewModel vm)> Open(string machine = "T1", string node = "FM1")
    {
        var h = await Harness.Create();
        h.Dialogs.Client = h.Client;
        return (h, await Select(h, machine, node));
    }

    private static async Task<MainViewModel> Select(Harness h, string machine, string node)
    {
        var vm = h.NewViewModel();
        await vm.InitializeAsync();
        vm.SelectedMachine = vm.Machines.First(m => m.Name == machine);
        vm.SelectedNode = node;
        await vm.ReloadAsync();
        return vm;
    }

    [Fact]
    public async Task ListsModelsAndFiltersCaseInsensitively()
    {
        await using var h = await Harness.Create();
        h.AddModel("T1", "FM1", "24fbo060-19", "a");
        h.AddModel("T1", "FM1", "7M99-535", "b");
        var vm = await Select(h, "T1", "FM1");
        Assert.Equal(2, vm.Models.Count);
        vm.SearchText = "FBO";
        Assert.Single(vm.Models);
        Assert.Equal("24fbo060-19", vm.Models[0].Name);
        Assert.Equal("1 / 2 models", vm.CountText);
        vm.SearchText = "zzz";
        Assert.Empty(vm.Models);
    }

    [Fact]
    public async Task CopyFm1ToFm2ReplacesTarget()
    {
        var (h, vm) = await Open();
        await using var _ = h;
        h.AddModel("T1", "FM1", "m1", "NEW");
        h.AddModel("T1", "FM2", "M1", "OLD");
        await vm.ReloadAsync();
        vm.SelectedModel = vm.Models.First();
        vm.SelectedSingleTarget = vm.SingleTargets.First(t => t.Machine == "T1" && t.Node == "FM2");
        Assert.True(vm.CopyCommand.CanExecute(null));

        await vm.CopyCommand.ExecuteAsync(null);

        Assert.Equal("done", h.Dialogs.LastJob?.State);
        Assert.Equal("handler NEW", h.Read("T1", "FM2", "PxRepository/m1/2.0/Line/Top/Top_handler.txt"));
        Assert.False(h.Exists("T1", "FM2", "PxRepository/M1"));
        Assert.StartsWith("Done", vm.StatusText);
    }

    [Fact]
    public async Task CopyAndRenameKeepsSourceAndRewritesHistory()
    {
        var (h, vm) = await Open();
        await using var _ = h;
        h.AddModel("T1", "FM1", "21csp043-09", "S");
        await vm.ReloadAsync();
        vm.SelectedModel = vm.Models.First();
        vm.SelectedSingleTarget = vm.SingleTargets.First(t => t.Node == "FM2");
        vm.CopyNewName = "21csp043-10";
        await vm.CopyCommand.ExecuteAsync(null);

        Assert.StartsWith("21csp043-10,", h.Read("T1", "FM2", "PxRepository/21csp043-10.his"));
        Assert.StartsWith("21csp043-09,", h.Read("T1", "FM1", "PxRepository/21csp043-09.his"));
        Assert.True(h.Exists("T1", "FM2", "PxInventory/INSPECT_SPEC/21csp043-10-00"));
    }

    [Fact]
    public async Task GroupCopyToOtherMachine()
    {
        var (h, vm) = await Open();
        await using var _ = h;
        foreach (var n in new[] { "FM1", "FM2", "BM" }) h.AddModel("T1", n, "g1", "T1-" + n);
        await vm.ReloadAsync();
        Assert.True(vm.CanGroupCopy);
        Assert.Equal(["T2"], vm.GroupTargets.ToArray());
        vm.SelectedModel = vm.Models.First();
        vm.CopyAsGroup = true;
        await vm.CopyCommand.ExecuteAsync(null);

        foreach (var n in new[] { "FM1", "FM2", "BM" })
            Assert.Equal("handler T1-" + n, h.Read("T2", n, "PxRepository/g1/2.0/Line/Top/Top_handler.txt"));
        Assert.Equal(3, h.Dialogs.Plans.Single().Units.Count);
    }

    [Fact]
    public async Task BmOffersOnlyOtherMachinesBm()
    {
        var (h, vm) = await Open("T1", "BM");
        await using var _ = h;
        Assert.Equal(["T2 / BM"], vm.SingleTargets.Select(t => t.ToString()).ToArray());
        vm.SelectedNode = "FM1";
        await vm.ReloadAsync();
        Assert.DoesNotContain(vm.SingleTargets, t => t.Node == "BM");
    }

    [Fact]
    public async Task RenameGroupRenamesAllThreeNodes()
    {
        var (h, vm) = await Open();
        await using var _ = h;
        foreach (var n in new[] { "FM1", "FM2", "BM" }) h.AddModel("T1", n, "old", n);
        await vm.ReloadAsync();
        vm.SelectedModel = vm.Models.First();
        vm.RenameAsGroup = true;
        vm.RenameNewName = "new";
        await vm.RenameCommand.ExecuteAsync(null);

        foreach (var n in new[] { "FM1", "FM2", "BM" })
        {
            Assert.True(h.Exists("T1", n, "PxRepository/new/2.0"));
            Assert.False(h.Exists("T1", n, "PxRepository/old"));
            Assert.StartsWith("new,", h.Read("T1", n, "PxRepository/new.his"));
        }
    }

    [Fact]
    public async Task RefusedPlansShowTheReasonAndChangeNothing()
    {
        var (h, vm) = await Open();
        await using var _ = h;
        h.AddModel("T1", "FM1", "m1", "S");
        h.AddModel("T1", "FM1", "other", "S");
        await vm.ReloadAsync();
        vm.SelectedModel = vm.Models.First(m => m.Name == "m1");
        vm.RenameNewName = "OTHER"; // collides case-insensitively
        await vm.RenameCommand.ExecuteAsync(null);
        Assert.Contains(h.Dialogs.Messages, m => m.StartsWith("E:") && m.Contains("already has a model"));
        Assert.True(h.Exists("T1", "FM1", "PxRepository/m1"));

        vm.RenameNewName = "bad/name";
        await vm.RenameCommand.ExecuteAsync(null);
        Assert.Contains(h.Dialogs.Messages, m => m.Contains("illegal character"));
    }

    [Fact]
    public async Task DecliningThePlanChangesNothing()
    {
        var (h, vm) = await Open();
        await using var _ = h;
        h.AddModel("T1", "FM1", "m1", "NEW");
        h.AddModel("T1", "FM2", "m1", "OLD");
        await vm.ReloadAsync();
        h.Dialogs.ApprovePlans = false;
        vm.SelectedModel = vm.Models.First();
        vm.SelectedSingleTarget = vm.SingleTargets.First(t => t.Node == "FM2");
        await vm.CopyCommand.ExecuteAsync(null);
        Assert.Equal("handler OLD", h.Read("T1", "FM2", "PxRepository/m1/2.0/Line/Top/Top_handler.txt"));
        Assert.Equal("Canceled", vm.StatusText);
        Assert.Contains(h.Dialogs.Plans.Single().Units[0].Steps, s => s.Danger); // the deletion was shown
    }

    [Fact]
    public async Task ExcelEntryIsReservedButNotImplemented()
    {
        var (h, vm) = await Open();
        await using var _ = h;
        await vm.ExportExcelCommand.ExecuteAsync(null);
        Assert.Contains(h.Dialogs.Messages, m => m.Contains("not implemented"));
    }

    [Fact]
    public async Task MachineCrudAndDefaultRestore()
    {
        await using var h = await Harness.Create();
        h.Dialogs.Client = h.Client;
        var vm = new MachinesViewModel(h.Client, h.Dialogs);
        await vm.LoadAsync();
        Assert.Equal(["T1", "T2"], vm.Items.Select(m => m.Name).ToArray());

        await vm.RestoreDefaultCommand.ExecuteAsync(null);
        var d = vm.Items.Single(m => m.Name == "AFVI 13");
        Assert.Equal(["192.168.1.61", "192.168.1.62", "192.168.1.63"], [d.Fm1.Host, d.Fm2.Host, d.Bm.Host]);

        vm.NewCommand.Execute(null);
        vm.Draft.Name = "AFVI 14";
        vm.Draft.Fm1.Host = "10.0.0.1"; vm.Draft.Fm2.Host = "10.0.0.2"; vm.Draft.Bm.Host = "10.0.0.3";
        await vm.SaveCommand.ExecuteAsync(null);
        Assert.False(vm.IsError, vm.Message);
        Assert.Contains(vm.Items, m => m.Name == "AFVI 14");

        vm.Draft.Name = "afvi 13"; // duplicate of the default, other case
        vm.NewCommand.Execute(null);
        vm.Draft.Name = "afvi 13";
        vm.Draft.Fm1.Host = vm.Draft.Fm2.Host = vm.Draft.Bm.Host = "x";
        await vm.SaveCommand.ExecuteAsync(null);
        Assert.True(vm.IsError);

        vm.Selected = vm.Items.First(m => m.Name == "AFVI 14");
        await vm.DeleteCommand.ExecuteAsync(null);
        Assert.DoesNotContain(vm.Items, m => m.Name == "AFVI 14");
        Assert.True(vm.Changed);
    }
}
