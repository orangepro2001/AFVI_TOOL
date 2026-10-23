using System.Net.Http.Json;
using System.Text;
using System.Text.Json;
using AfviTools.Localization;
using AfviTools.Models;

namespace AfviTools.Services;

/// <summary>An error reported by the core (HTTP status plus the message the user should see).</summary>
public class CoreException(string message, int status) : Exception(message)
{
    public int Status { get; } = status;
}

/// <summary>Typed client of the afvi-core HTTP API.</summary>
public sealed class CoreClient : IDisposable
{
    private readonly HttpClient _http;

    public Uri BaseAddress => _http.BaseAddress!;

    public CoreClient(Uri baseAddress, string token)
    {
        _http = new HttpClient { BaseAddress = baseAddress, Timeout = TimeSpan.FromMinutes(10) };
        _http.DefaultRequestHeaders.Add("X-Afvi-Token", token);
    }

    public void Dispose() => _http.Dispose();

    private static string Esc(string s) => Uri.EscapeDataString(s);

    private async Task<T> Send<T>(HttpMethod method, string path, object? body, System.Text.Json.Serialization.Metadata.JsonTypeInfo<T> info,
        CancellationToken ct)
    {
        using var req = new HttpRequestMessage(method, path);
        req.Headers.Add("X-Afvi-Lang", Loc.Instance.Tag); // the core words its plans and refusals in the GUI language
        if (body != null)
        {
            var typeInfo = WireJson.Default.GetTypeInfo(body.GetType())
                           ?? throw new InvalidOperationException("no JSON metadata for " + body.GetType());
            req.Content = new StringContent(JsonSerializer.Serialize(body, typeInfo), Encoding.UTF8, "application/json");
        }
        using var resp = await _http.SendAsync(req, ct);
        var text = await resp.Content.ReadAsStringAsync(ct);
        if (!resp.IsSuccessStatusCode)
        {
            string msg;
            try { msg = JsonSerializer.Deserialize(text, WireJson.Default.ErrorDto)?.Error ?? text; }
            catch (JsonException) { msg = text; }
            throw new CoreException(msg, (int)resp.StatusCode);
        }
        return JsonSerializer.Deserialize(text, info) ?? throw new CoreException("empty response", (int)resp.StatusCode);
    }

    public Task<HealthDto> Health(CancellationToken ct = default) =>
        Send(HttpMethod.Get, "/v1/health", null, WireJson.Default.HealthDto, ct);

    public Task<ConfigDto> GetConfig(CancellationToken ct = default) =>
        Send(HttpMethod.Get, "/v1/config", null, WireJson.Default.ConfigDto, ct);

    public Task<MachineDto> AddMachine(MachineDto m, CancellationToken ct = default) =>
        Send(HttpMethod.Post, "/v1/machines", m, WireJson.Default.MachineDto, ct);

    public Task<MachineDto> UpdateMachine(string oldName, MachineDto m, CancellationToken ct = default) =>
        Send(HttpMethod.Put, "/v1/machines/" + Esc(oldName), m, WireJson.Default.MachineDto, ct);

    public Task DeleteMachine(string name, CancellationToken ct = default) =>
        Send(HttpMethod.Delete, "/v1/machines/" + Esc(name), null, WireJson.Default.DictionaryStringBoolean, ct);

    public Task RestoreDefaults(CancellationToken ct = default) =>
        Send(HttpMethod.Post, "/v1/machines/restore-defaults", null, WireJson.Default.DictionaryStringBoolean, ct);

    public Task<ModelListDto> ListModels(string machine, string node, CancellationToken ct = default) =>
        Send(HttpMethod.Get, $"/v1/machines/{Esc(machine)}/nodes/{Esc(node)}/models", null, WireJson.Default.ModelListDto, ct);

    public Task<TargetsDto> Targets(string machine, string node, CancellationToken ct = default) =>
        Send(HttpMethod.Get, $"/v1/policy/targets?machine={Esc(machine)}&node={Esc(node)}", null, WireJson.Default.TargetsDto, ct);

    public Task<NameCheckDto> CheckName(string name, CancellationToken ct = default) =>
        Send(HttpMethod.Post, "/v1/names/check", new Dictionary<string, string> { ["name"] = name }, WireJson.Default.NameCheckDto, ct);

    public Task<PlanDto> PlanCopy(CopyRequestDto r, CancellationToken ct = default) =>
        Send(HttpMethod.Post, "/v1/plans/copy", r, WireJson.Default.PlanDto, ct);

    public Task<PlanDto> PlanRename(RenameRequestDto r, CancellationToken ct = default) =>
        Send(HttpMethod.Post, "/v1/plans/rename", r, WireJson.Default.PlanDto, ct);

    public Task<JobDto> StartJob(string planId, CancellationToken ct = default) =>
        Send(HttpMethod.Post, "/v1/jobs", new StartJobDto(planId), WireJson.Default.JobDto, ct);

    public Task<JobDto> GetJob(string id, CancellationToken ct = default) =>
        Send(HttpMethod.Get, "/v1/jobs/" + Esc(id), null, WireJson.Default.JobDto, ct);

    public Task CancelJob(string id, CancellationToken ct = default) =>
        Send(HttpMethod.Post, $"/v1/jobs/{Esc(id)}/cancel", null, WireJson.Default.DictionaryStringBoolean, ct);

    /// <summary>The Excel export entry. The core answers 501 until it is implemented.</summary>
    public Task<Dictionary<string, string>> ExportExcel(CancellationToken ct = default) =>
        Send(HttpMethod.Post, "/v1/export/excel", new Dictionary<string, string>(), WireJson.Default.DictionaryStringString, ct);
}
