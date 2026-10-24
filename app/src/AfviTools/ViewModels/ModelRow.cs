using System.Diagnostics.CodeAnalysis;
using System.Globalization;
using AfviTools.Localization;
using AfviTools.Models;

namespace AfviTools.ViewModels;

/// <summary>
/// One line of the model list. The DataGrid sorts columns through property-path reflection, so the properties are
/// kept for trimming and Native AOT (without this, header sorting silently does nothing in an AOT build).
/// </summary>
[DynamicallyAccessedMembers(DynamicallyAccessedMemberTypes.PublicProperties)]
public sealed class ModelRow
{
    public ModelRow(ModelRowDto d)
    {
        Name = d.Name;
        Class = d.Class;
        var yes = Loc.Instance["yes_mark"];
        var no = Loc.Instance["no_mark"];
        Repository = d.Repo ? yes : no;
        History = d.History ? yes : no;
        Inspect = d.Inspect == 0 ? no : d.Inspect.ToString(CultureInfo.InvariantCulture);
        Light = d.Light ? yes : no;
        Modified = DateTimeOffset.TryParse(d.ModTime, CultureInfo.InvariantCulture, DateTimeStyles.None, out var t)
            ? t.ToLocalTime().ToString("yyyy-MM-dd HH:mm", CultureInfo.InvariantCulture)
            : "";
        Complete = d.Repo && d.Light && d.Inspect > 0;
    }

    public string Name { get; }
    public string Class { get; }
    public string Repository { get; }
    public string History { get; }
    public string Inspect { get; }
    public string Light { get; }
    public string Modified { get; }

    /// <summary>True when the model has its repository, an INSPECT_SPEC and a LIGHT_SPEC.</summary>
    public bool Complete { get; }
}
