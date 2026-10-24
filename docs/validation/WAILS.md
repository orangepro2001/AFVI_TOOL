# Validation: Wails + in-process RPC (branch `wails-rpc`)

Question: can the tool use Wails v2 (Go + system webview), with the Go core bound directly to the front end (no sidecar process,
no port, no token), and what does it cost or save compared with the C# + Avalonia + Go sidecar version on `main`?

Date 2026-10-24, Linux dev machine (Xvfb), Wails 2.17.0, webkit2gtk 2.52 (tag `webkit2_41`). Nothing was run on Windows or macOS.

## What was built

- `core/internal/service`: the use cases as plain Go methods (the former HTTP handlers' logic, moved). `internal/server` is now a thin HTTP
  transport over it; its tests pass unchanged, so the sidecar version still works.
- `core/internal/wailsapp`: the object bound to the front end (language is the first argument of every method that words text). Pure Go, unit tested.
- `core/cmd/afvi-wails`: Wails entry (`//go:build desktop`, so plain `go build ./...` ignores it) and a front end in **plain HTML/JS/CSS, no Node, no
  build step** (`frontend/dist`): the same screens and flows as the Avalonia GUI (list, search, copy single/group, rename single/group, plan dialog,
  job dialog with progress, four languages with live switch, font size, Excel entry, status bar), the same language packs (copied JSON), the
  `win2000.css` port, and a **drawn Windows 2000 caption bar** with minimise/maximise/close in a frameless window.
  Not ported: the machine configuration dialog (the bound methods exist).
- `-selftest=<dir>`: builds throw-away local machines, loads `selftest.js`, which drives the real window through the DOM (list, search, every language,
  copy with the red deletion step in the plan, group rename, refused plan, Excel entry) and asks Go to verify the files. `scripts/wails.sh` builds and runs it.

## Results

| | Wails + RPC (this branch) | Avalonia + Go sidecar (`main`) |
| --- | --- | --- |
| Windows binary | **12 MB** (one file, cross-built from Linux in 4 s, no cgo) | 24 MB (one file, trimmed, core embedded) |
| Linux binary | 10 MB (needs libwebkit2gtk-4.1) | 25 MB |
| macOS | not built (needs a Mac host) | 26 MB (cross-built, not run) |
| Idle RSS, Linux | **~460 MB** (3 processes: app + WebKit network + WebKit web process) | **~208 MB** (app + core) |
| Window on screen after launch (Xvfb, rough) | ~0.25 s | ~1.2 s |
| Whole scripted UI journey (self-test) | 1.7 s | n/a |
| GUI source (hand written) | 320 lines JS + 142 HTML + 52 CSS | 1,964 lines C# + XAML |
| Extra processes / ports / tokens | none | child process, loopback port, token, stdin watch, embedded-core unpacking |
| Toolchain | Go + wails CLI (no Node needed for this front end) | Go + .NET SDK |
| Go tests / UI test | 8 Go packages, UI self-test in a real window | Go + 25 C# tests, `--selftest` |

Reading the numbers:

- **Disk**: half the size, and Windows needs no unpacking at first start. This is the only place Wails wins by a lot.
- **Memory**: Wails is more than twice as heavy. WebKit runs as several processes; the existing Wails app (`AFVI-UI`) measured the same (515 MB).
  On Windows the WebView2 runtime is a shared Edge component with a similar multi-process footprint (not measured here).
- **The Windows 2000 look**: reproduced well by CSS (bevels, group boxes, tabs, tables, menus all come from the skin's CSS) and the drawn caption
  works in the frameless window. On Linux without a window manager the window has no resize handles; **frameless resize/drag, the DWM shadow and Windows 11's rounded
  corners must be checked on Windows 10 and 11** (Wails' frameless support differs per OS). The `-native-frame` flag falls back to the system title bar.

## What got simpler, what got harder

Simpler: no process lifecycle, no JSON DTO duplication (types are Go structs; the JS sees the same JSON), errors are plain rejected promises with the
already translated text, the language packs are shared as-is, one binary, instant start. The service extraction also improved the core.

Harder: no native controls (every widget is CSS; combos, tables and dialogs are hand-built in a few hundred lines, which the Avalonia theme gave for free),
HTML/JS has no compile-time checking (the self-test is what catches mistakes), menus/dialogs/keyboard handling are our own code, and the three platforms need three
build hosts for release (Windows can be cross-built; Linux needs webkit2gtk headers; macOS needs a Mac).

## Not verified (needs a Windows machine)

1. WebView2 present on the target machines (Windows 10+ only; offline PCs need the bootstrapper in an NSIS installer: `wails build -nsis`).
2. Frameless caption on Windows 10/11: dragging, double-click maximise, resize edges, snap layouts, Win11 rounded corners and shadow.
3. Korean/Chinese IME input in the text fields under WebView2 (expected fine: it is the system text engine).
4. Real memory and start time under WebView2.
5. Pixel comparison of fonts: Tahoma exists on Windows, so the look there is closer than the Linux fallback screenshots.

## Verdict: go, with conditions

Wails + RPC works for this tool and removes the whole sidecar layer. Choose it over the Avalonia build if (a) the target machines run Windows 10 or newer with
(or can install) WebView2, and (b) disk size and a single small file matter more than idle memory. Keep Avalonia if native-widget behaviour (text editing,
accessibility, DataGrid features) or lower memory matter more.

Effort to production grade: about 2-3 days: port the machine dialog, keyboard shortcuts and focus handling, column sorting, an NSIS installer with the WebView2
bootstrapper, a Windows smoke test in CI (`windows-latest`: start the exe, run `-selftest`), and the Windows 10/11 frame check above.

Build: `scripts/wails.sh` (needs `wails` v2.17.0 on PATH, `libwebkit2gtk-4.1-dev`, `xvfb` for the self-test).
