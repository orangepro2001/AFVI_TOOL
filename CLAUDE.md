# CLAUDE.md

AFVI TOOLS: Model Copy + Export Data To Excel helper for AFVI machines; also a feasibility study of C# (Avalonia) + Go
as a cross-platform app with a Windows 2000 look. Developed on Linux, shipped on Windows. User-facing docs are Chinese.

## Hard rules

- **Dev phase: the real machines (192.168.1.61/62/63, shares PxRepository/PxInventory, user `pixel`, empty password) are read-only.**
  Never write, rename or delete there. Test copies on `LOCAL TEST` (`scripts/dev-local.sh`) or on temp dirs. The vfs layer enforces it
  (`vfs.Open` returns read-only file systems for network paths unless `allowNetworkWrite`); do not bypass it, do not pass `-allow-network-write`.
- No commit/push unless asked. Repo: `git@github.com:orangepro2001/AFVI_TOOL.git` (remote is configured, nothing pushed).
- `.local-mirror/` holds copies of customer data: git-ignored, never commit it.
- The Windows 2000 look comes from Classic.Avalonia.Theme plus `app/src/AfviTools/Styles/Win2000.axaml` (tokens of the win2000-style skill). No rounded corners, shadows or hover effects. Verify with a rendered screenshot, not by reading XAML.

## Commands

```bash
scripts/build.sh [--no-test] [--platform win|linux|mac|all]   # the one-click check; native apps go to ./build/<platform> (ONE file each: AfviTools[.exe] with the runtime and the Go core embedded, compressed and trimmed, ~25 MB); dotnet is looked up in PATH then ~/.dotnet
cd core && go test ./...                   # Go only (pure Go, no cgo)
dotnet test app/tests/AfviTools.Tests      # rebuilds bin/afvi-core first, starts it for real
scripts/run-dev.sh [--lang=ko|zh-Hans|zh-Hant|en] [--fontsize=N] [--config=FILE]
scripts/screenshot.sh shots/x.png [app args]   # Xvfb render; dotnet test also writes shots/ui-*.png
command gofmt -l core                      # `gofmt` may be aliased to a rewriting tool in zsh
```

`dotnet` is installed user-level in `~/.dotnet` (not on PATH by default): `export PATH=$HOME/.dotnet:$PATH DOTNET_ROOT=$HOME/.dotnet`.

## Where things are

See README.md (layout) and docs/ARCHITECTURE.md (process model, API, copy/rename semantics), docs/NAMING.md (what a model is on the machines).
Copy/rename rules live in one place, `core/internal/policy`. Languages: default Korean; core packs in `core/internal/i18n/catalog.go`,
GUI packs in `app/src/AfviTools/Assets/lang/*.json` (flat key -> text; same keys and `{0}` placeholders in all four).

## Gotchas learned

- Packaging: `PublishSingleFile` + `IncludeNativeLibrariesForSelfExtract` + `EnableCompressionInSingleFile` + `PublishTrimmed` (TrimMode=partial) took the win-x64 app from 99 MB to 25 MB. The Go core is an `EmbeddedResource` (`/p:EmbeddedCore=`), unpacked by `CoreHost.ExtractEmbedded` into `UiSettings.DataDirectory()`. Trimmed builds can break only at run time: `--selftest=<dir>` opens every window in every language (build.sh runs it on the Linux package).
- `Environment.GetFolderPath(...)` can return "" when the folder does not exist, which silently writes into the working directory; use `UiSettings.DataDirectory()`.
- In zsh an unquoted `$VAR` holding several options is not word-split; put multi-option commands in a bash script.

- Avalonia indexer bindings (`{loc:T key}`) refresh only when `PropertyChanged("Item")` is raised, not just `"Item[]"`.
- `TextTrimming="PathSegmentTrimming"` is not valid in Avalonia 11.3 (runtime crash only when the window opens); the headless render tests open every window.
- A `ColumnDefinition Width="Auto"` for the right panel made the DataGrid columns collapse; the panel width is set from code (`ModelCopyView`).
- `dotnet build -v q` hides XAML compile errors in a `| tail`; check the exit code.
- `Application.Current` is null during `App.Initialize()`; look settings go through `UiSettings.Host`.
