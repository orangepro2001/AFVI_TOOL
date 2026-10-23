using System.Text.Json.Serialization;

namespace AfviTools.Models;

// Wire types of the afvi-core API (core/internal/server). Names are camelCase
// on the wire; the source generator below keeps this AOT/trim friendly.

public record NodeRef(string Machine, string Node)
{
    public override string ToString() => $"{Machine} / {Node}";
}

public class NodeDto
{
    public string Host { get; set; } = "";
    public string? RepositoryPath { get; set; }
    public string? InventoryPath { get; set; }

    public NodeDto Clone() => new() { Host = Host, RepositoryPath = RepositoryPath, InventoryPath = InventoryPath };
}

public class MachineDto
{
    public string Name { get; set; } = "";
    public NodeDto Fm1 { get; set; } = new();
    public NodeDto Fm2 { get; set; } = new();
    public NodeDto Bm { get; set; } = new();

    public MachineDto Clone() => new() { Name = Name, Fm1 = Fm1.Clone(), Fm2 = Fm2.Clone(), Bm = Bm.Clone() };
    public override string ToString() => Name;
}

public record SettingsDto(string SmbUser, bool AllowNetworkWrite);

public record ConfigDto(List<MachineDto> Machines, SettingsDto Settings, List<string> Kinds);

public record HealthDto(string Version, string Os, bool AllowNetworkWrite, string ConfigPath);

public record ModelRowDto(string Name, string Class, bool Repo, bool History, int Inspect, bool Light, string? ModTime);

public record ModelListDto(List<ModelRowDto> Models, bool ReadOnly, string Repository, string Inventory);

public record TargetsDto(List<NodeRef> Single, List<string> Group);

public record NameCheckDto(bool Ok, string? Error, string Class);

public record CopyRequestDto(string Mode, NodeRef Source, NodeRef Target, string Model, string NewName);

public record RenameRequestDto(string Mode, NodeRef Ref, string Model, string NewName);

public record StepDto(string Action, string Text, bool Danger);

public record UnitDto(NodeRef? Source, NodeRef Target, string Model, string NewName, List<StepDto> Steps, int Files, long Bytes);

public record PlanDto(string Id, string Kind, string Title, List<UnitDto> Units, List<string> Warnings, List<string> Blockers, int Files, long Bytes);

public record StartJobDto(string PlanId);

public record ProgressDto(string Phase, string Current, int FilesDone, int FilesTotal, long BytesDone, long BytesTotal,
    bool Committed, List<string> Log, double Seconds);

public record JobDto(string Id, string Title, string State, string? Error, ProgressDto Progress);

public record ErrorDto(string Error);

[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase,
    DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull)]
[JsonSerializable(typeof(NodeRef))]
[JsonSerializable(typeof(MachineDto))]
[JsonSerializable(typeof(ConfigDto))]
[JsonSerializable(typeof(HealthDto))]
[JsonSerializable(typeof(ModelListDto))]
[JsonSerializable(typeof(TargetsDto))]
[JsonSerializable(typeof(NameCheckDto))]
[JsonSerializable(typeof(CopyRequestDto))]
[JsonSerializable(typeof(RenameRequestDto))]
[JsonSerializable(typeof(PlanDto))]
[JsonSerializable(typeof(StartJobDto))]
[JsonSerializable(typeof(JobDto))]
[JsonSerializable(typeof(ErrorDto))]
[JsonSerializable(typeof(Dictionary<string, string>))]
[JsonSerializable(typeof(Dictionary<string, bool>))]
internal partial class WireJson : JsonSerializerContext;
