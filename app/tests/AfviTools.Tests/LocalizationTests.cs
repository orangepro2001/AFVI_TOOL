using System.Text.RegularExpressions;
using AfviTools.Localization;
using AfviTools.Models;
using AfviTools.Services;
using AfviTools.ViewModels;

namespace AfviTools.Tests;

public class LocalizationTests
{
    private static readonly string[] Tags = ["ko", "zh-Hans", "zh-Hant", "en"];

    [Fact]
    public void DefaultLanguageIsKorean()
    {
        Assert.Equal("ko", Loc.DefaultTag);
        Assert.Equal("ko", Loc.Instance.Resolve(null));
        Assert.Equal("ko", Loc.Instance.Resolve(""));
        Assert.Equal("ko", Loc.Instance.Resolve("fr-FR"));
    }

    [Fact]
    public void ResolvesLocaleTags()
    {
        var cases = new Dictionary<string, string>
        {
            ["ko-KR"] = "ko", ["en-US"] = "en", ["zh-CN"] = "zh-Hans", ["zh"] = "zh-Hans",
            ["zh-TW"] = "zh-Hant", ["zh-HK"] = "zh-Hant", ["zh-Hant-TW"] = "zh-Hant", ["zh-hans"] = "zh-Hans",
        };
        foreach (var (input, want) in cases) Assert.Equal(want, Loc.Instance.Resolve(input));
    }

    [Fact]
    public void FourPacksAreCompleteWithSamePlaceholders()
    {
        var keys = Loc.Instance.Keys.ToList();
        Assert.True(keys.Count > 100);
        var ph = new Regex(@"\{\d+\}");
        foreach (var key in keys)
        {
            var want = string.Join(",", ph.Matches(Loc.Instance.Pack("en")[key]).Select(m => m.Value).Order());
            foreach (var tag in Tags)
            {
                Assert.True(Loc.Instance.Pack(tag).TryGetValue(key, out var text) && !string.IsNullOrEmpty(text), $"{tag} lacks {key}");
                var got = string.Join(",", ph.Matches(text!).Select(m => m.Value).Order());
                Assert.True(got == want, $"{tag}/{key}: placeholders [{got}] vs English [{want}]");
            }
        }
    }

    [Fact]
    public void EveryKeyUsedInXamlAndCodeExists()
    {
        var root = AppContext.BaseDirectory;
        while (root != null && !Directory.Exists(Path.Combine(root, "app", "src"))) root = Path.GetDirectoryName(root);
        Assert.NotNull(root);
        var src = Path.Combine(root!, "app", "src", "AfviTools");
        var used = new HashSet<string>();
        foreach (var f in Directory.EnumerateFiles(src, "*.axaml", SearchOption.AllDirectories))
            foreach (Match m in Regex.Matches(File.ReadAllText(f), @"\{loc:T (\w+)\}")) used.Add(m.Groups[1].Value);
        foreach (var f in Directory.EnumerateFiles(src, "*.cs", SearchOption.AllDirectories))
            foreach (Match m in Regex.Matches(File.ReadAllText(f), @"(?:Loc\.Instance|\bL)(?:\.Format\(|\[)""(\w+)""")) used.Add(m.Groups[1].Value);
        Assert.True(used.Count > 80, "found only " + used.Count);
        var known = Loc.Instance.Keys.ToHashSet();
        Assert.Empty(used.Where(k => !known.Contains(k)));
    }

    [Fact]
    public void SwitchingChangesTextsAndRaisesNotifications()
    {
        Loc.Instance.Set("ko");
        var changed = 0;
        void Handler() => changed++;
        Loc.Instance.LanguageChanged += Handler;
        try
        {
            Assert.Equal("파일(_F)", Loc.Instance["menu_file"]);
            Loc.Instance.Set("zh-Hant");
            Assert.Equal("檔案(_F)", Loc.Instance["menu_file"]);
            Loc.Instance.Set("zh-Hans");
            Assert.Equal("文件(_F)", Loc.Instance["menu_file"]);
            Loc.Instance.Set("en");
            Assert.Equal("_File", Loc.Instance["menu_file"]);
            Assert.Equal(3, changed);
            Loc.Instance.Set("en"); // no change, no event
            Assert.Equal(3, changed);
            Assert.Equal("no.such.key", Loc.Instance["no.such.key"]);
            Assert.Equal("Source: A / FM1", Loc.Instance.Format("lbl_source_is", "A / FM1"));
        }
        finally { Loc.Instance.LanguageChanged -= Handler; Loc.Instance.Set("en"); }
    }

    [Fact]
    public void ExternalPackAddsALanguageAndOverridesKeys()
    {
        var dir = Path.Combine(Path.GetTempPath(), "afvi-lang-" + Guid.NewGuid().ToString("N")[..8]);
        Directory.CreateDirectory(dir);
        try
        {
            File.WriteAllText(Path.Combine(dir, "ja.json"), "{\"_name\":\"日本語\",\"menu_file\":\"ファイル(_F)\"}");
            File.WriteAllText(Path.Combine(dir, "broken.json"), "{ not json");
            File.WriteAllText(Path.Combine(dir, "en.json"), "{\"btn_close\":\"Close window\"}");
            Loc.Instance.LoadExternal(dir);
            Assert.Contains(Loc.Instance.Languages, l => l.Tag == "ja" && l.NativeName == "日本語");
            Loc.Instance.Set("ja");
            Assert.Equal("ファイル(_F)", Loc.Instance["menu_file"]);
            Assert.Equal("Close window", Loc.Instance["btn_close"]); // a key the ja pack lacks falls back to the (overridden) English
        }
        finally { Loc.Instance.Set("en"); try { Directory.Delete(dir, true); } catch (IOException) { } }
    }

    [Fact]
    public async Task CoreWordsItsMessagesInTheGuiLanguage()
    {
        await using var h = await Harness.Create("ko");
        h.AddModel("T1", "FM1", "m1", "x");
        var req = new CopyRequestDto("single", new NodeRef("T1", "FM1"), new NodeRef("T1", "BM"), "m1", "");
        var expected = new Dictionary<string, string>
        {
            ["ko"] = "FM과 BM 사이에는 복사할 수 없습니다",
            ["zh-Hans"] = "FM 与 BM 之间不允许复制",
            ["zh-Hant"] = "FM 與 BM 之間不允許複製",
            ["en"] = "Copying between FM and BM is not allowed",
        };
        foreach (var (tag, text) in expected)
        {
            Loc.Instance.Set(tag);
            var ex = await Assert.ThrowsAsync<CoreException>(() => h.Client.PlanCopy(req));
            Assert.Equal(text, ex.Message);
        }
        Loc.Instance.Set("ko");
        var plan = await h.Client.PlanCopy(new CopyRequestDto("single", new NodeRef("T1", "FM1"), new NodeRef("T1", "FM2"), "m1", ""));
        Assert.StartsWith("복사 m1", plan.Title);
        Assert.Contains(plan.Units[0].Steps, s => s.Text.StartsWith("복사 "));
    }

    [Fact]
    public async Task ModelListRebuildsInTheNewLanguage()
    {
        await using var h = await Harness.Create("ko");
        h.AddModel("T1", "FM1", "m1", "x");
        var vm = h.NewViewModel();
        await vm.InitializeAsync();
        vm.SelectedMachine = vm.Machines.First(m => m.Name == "T1");
        await vm.ReloadAsync();
        Assert.Equal("있음", vm.Models[0].Repository);
        Assert.Equal("모델 1개", vm.CountText);
        Assert.Contains("쓰기", vm.LockText);
        Loc.Instance.Set("zh-Hans");
        Assert.Equal("有", vm.Models[0].Repository);
        Assert.Equal("1 个型号", vm.CountText);
        Assert.Contains("可写", vm.LockText);
        Loc.Instance.Set("en");
        Assert.Equal("yes", vm.Models[0].Repository);
        Assert.Equal("Source: T1 / FM1", vm.SourceLabel);
    }
}
