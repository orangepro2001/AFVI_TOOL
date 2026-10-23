using AfviTools.Localization;
using AfviTools.Models;
using AfviTools.Services;
using AfviTools.ViewModels;

namespace AfviTools.Tests;

/// <summary>A real afvi-core process on a temp config with two local machines (T1, T2); no network.</summary>
public sealed class Harness : IAsyncDisposable
{
    public string Root { get; } = Path.Combine(Path.GetTempPath(), "afvi-tests-" + Guid.NewGuid().ToString("N")[..8]);
    public CoreHost Host { get; private set; } = null!;
    public CoreClient Client => Host.Client;
    public FakeDialogs Dialogs { get; } = new();

    public static async Task<Harness> Create(string language = "en")
    {
        Loc.Instance.Set(language); // text assertions are made in English unless a test says otherwise
        var h = new Harness();
        Directory.CreateDirectory(h.Root);
        h.Host = await CoreHost.Start(Path.Combine(h.Root, "config.json"), false);
        await h.Client.DeleteMachine("AFVI 13"); // never touch the network from tests
        await h.Client.AddMachine(h.Machine("T1"));
        await h.Client.AddMachine(h.Machine("T2"));
        return h;
    }

    public string NodeDir(string machine, string node) => Path.Combine(Root, machine, node);

    private MachineDto Machine(string name)
    {
        NodeDto Node(string kind)
        {
            var d = NodeDir(name, kind);
            Directory.CreateDirectory(Path.Combine(d, "PxRepository"));
            Directory.CreateDirectory(Path.Combine(d, "PxInventory"));
            return new NodeDto { Host = "local", RepositoryPath = Path.Combine(d, "PxRepository"), InventoryPath = Path.Combine(d, "PxInventory") };
        }
        return new MachineDto { Name = name, Fm1 = Node("FM1"), Fm2 = Node("FM2"), Bm = Node("BM") };
    }

    /// <summary>Creates a complete model on a node: repository tree, .his, INSPECT_SPEC-00, LIGHT_SPEC.</summary>
    public void AddModel(string machine, string node, string name, string tag)
    {
        var d = NodeDir(machine, node);
        Write(d, $"PxRepository/{name}/2.0/Line/Top/Top_handler.txt", "handler " + tag);
        Write(d, $"PxRepository/{name}/2.0/Line/Top/Layer/L01/1.tif", "tif " + tag);
        Write(d, $"PxRepository/{name}.his", $"{name},0,0,1,Admin,20260507114036\r\n");
        Write(d, $"PxInventory/INSPECT_SPEC/{name}-00/TOP/LIGHT0/InspectionSpec.xml", "spec " + tag);
        Write(d, $"PxInventory/LIGHT_SPEC/{name}/LightSpec.xml", "light " + tag);
    }

    public static void Write(string dir, string rel, string text)
    {
        var p = Path.Combine(dir, rel);
        Directory.CreateDirectory(Path.GetDirectoryName(p)!);
        File.WriteAllText(p, text);
    }

    public string Read(string machine, string node, string rel) => File.ReadAllText(Path.Combine(NodeDir(machine, node), rel));
    public bool Exists(string machine, string node, string rel) =>
        File.Exists(Path.Combine(NodeDir(machine, node), rel)) || Directory.Exists(Path.Combine(NodeDir(machine, node), rel));

    public MainViewModel NewViewModel() => new(Client, Dialogs);

    public ValueTask DisposeAsync()
    {
        Host.Dispose();
        try { Directory.Delete(Root, true); } catch (IOException) { }
        return ValueTask.CompletedTask;
    }
}

/// <summary>Records what the view model asks the user and answers automatically.</summary>
public sealed class FakeDialogs : IDialogService
{
    public bool ApprovePlans { get; set; } = true;
    public bool ConfirmAnswer { get; set; } = true;
    public List<PlanDto> Plans { get; } = [];
    public List<string> Messages { get; } = [];
    public JobDto? LastJob { get; private set; }
    public Func<Task<bool>>? OnEditMachines { get; set; }

    public Task ShowMessage(string title, string text, bool error = false)
    {
        Messages.Add((error ? "E:" : "I:") + title + ": " + text);
        return Task.CompletedTask;
    }

    public Task<bool> Confirm(string title, string text) => Task.FromResult(ConfirmAnswer);

    public Task<bool> ConfirmPlan(PlanDto plan)
    {
        Plans.Add(plan);
        return Task.FromResult(ApprovePlans && plan.Blockers.Count == 0);
    }

    public CoreClient? Client { get; set; }

    public async Task<JobDto> ShowJob(JobDto job)
    {
        var vm = new JobViewModel(Client!, job);
        using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(30));
        await vm.Poll(cts.Token);
        LastJob = vm.Final;
        return vm.Final;
    }

    public Task<bool> EditMachines() => OnEditMachines?.Invoke() ?? Task.FromResult(false);
}
