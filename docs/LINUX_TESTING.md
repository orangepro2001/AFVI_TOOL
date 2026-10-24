# Testing a Windows-only Win32 app on Linux

The app is Windows only (C GUI + Go core, cross-built with mingw-w64), but development and CI run on Linux. Nothing here replaces a smoke test on
real Windows (see "What Linux cannot tell you"); it makes sure that almost everything that can break is caught before that.

## Three layers, three kinds of test

| Layer | What it holds | How it is tested on Linux |
| --- | --- | --- |
| Go core | copy/rename rules, plans, jobs, vfs read-only protection, language packs | `cd core && go test ./...` (pure Go, temp directories, never the real machines) |
| C ABI | `afvi_init / afvi_call / afvi_free`, JSON in and out, panics turned into errors | `core/internal/capi` is plain Go behind the cgo file: unit tests run the same journey as the GUI (list, plan, job, verify files) |
| GUI | message handling, layout, drawing | preview renderer + (planned) logic self-test + snapshot comparison, below |

## 1. The preview renderer (exists)

`gui/gfx.h` is a five-function drawing interface (`fill`, `gradient`, `text`, `text_w`, `clip`). `gui/skin.c` and `gui/app.c` draw only through it. Two backends:
GDI (`gui/win_main.c`, Windows) and a software canvas that writes PNG (`gui/preview/preview_main.c`, Linux). `scripts/win32.sh` builds the Linux preview with the
host `gcc` against the **real Go core** (`go build -buildmode=c-archive` for linux) on throw-away machines, then renders: copy plan, job, menus, combos, refusals,
four languages. Run `scripts/win32.sh` and look at `shots/*.png` (reference copies in `docs/validation/win32-preview/`).

What it proves: layout, colours, bevels, text clipping, string lengths in four languages, the whole app logic against the real core. What it does not: GDI
specifics (font metrics, ClearType, DPI), real window messages, IME, frameless caption.

## 2. Snapshot regression (to build)

1. Render each screen to PNG with a fixed font size, fixed language, fixed fixture data and a bundled font (DejaVu Sans + Noto Sans CJK, never the system default,
   so CI and a laptop render alike).
2. Keep the PNGs under `tests/snapshots/` as the baseline; the test renders again and compares pixel by pixel (exact match; the renderer is deterministic).
3. A change in look shows up as a failing image diff in the pull request: either a bug, or a deliberate change whose new baseline is committed.
Cost: about half a day (render list, compare tool, CI step).

## 3. Logic self-test without a window system (to build)

Split `app.c` into (a) state and event handling that never calls Win32 (selection, search filter, plan/job flow, language) and (b) painting and the message loop.
Then tests on Linux feed events (`select row`, `click Copy`, `confirm plan`) to (a), step the job, and assert both the UI state and the files on disk via the core,
exactly the journey of the earlier `-selftest` (list, search, every language, copy with the deletion step shown, group rename, refused plan, Excel entry).
The Windows `-selftest <dir>` flag can reuse the same script on a real window later. Cost: 1 to 2 days of refactoring.

## 4. Wine (optional)

User-level Wine (`apt download` + `dpkg -x`, bubblewrap) started its server and prefix, but its `user32.dll` was a stub (`CreateDialogParamW` unimplemented), so
no program ran. A full Wine from the distribution package (root or a container) is the closest thing to running the real exe on Linux (GDI text, messages, the
`-selftest`); try it where root or Docker is available. Wine's default look is Windows 2000-like, which does not prove the Windows 10/11 look.

## What Linux cannot tell you (needs a half-day Windows smoke test)

Start-up, GDI font fallback (Tahoma, Malgun Gothic, YaHei, JhengHei), the EDIT controls' look and Korean/Chinese IME, frameless caption on Windows 10/11 (corners,
maximise over the taskbar), DPI changes, idle memory, antivirus behaviour for a Go+cgo exe. CI placeholder `smoke-windows` in `.github/workflows/ci.yml` is where the
automated version goes (`windows-latest`, run `AfviTools.exe -selftest <dir>`).
