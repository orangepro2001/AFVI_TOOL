# Testing a Windows-only Win32 app on Linux

The app is Windows only (C GUI + Go core, cross-built with mingw-w64), but development and CI run on Linux. Nothing here replaces a smoke test on
real Windows (see "What Linux cannot tell you"); it makes sure that almost everything that can break is caught before that.

## Three layers, three kinds of test

| Layer | What it holds | How it is tested on Linux |
| --- | --- | --- |
| Go core | copy/rename rules, plans, jobs, vfs read-only protection, language packs | `cd core && go test ./...` (pure Go, temp directories, never the real machines) |
| C ABI | `afvi_init / afvi_call / afvi_free`, JSON in and out, panics turned into errors | `core/internal/capi` is plain Go behind the cgo file: unit tests run the same journey as the GUI (list, plan, job, verify files) |
| GUI | message handling, layout, drawing | `scripts/test-gui.sh`: scripted UI journey + snapshot comparison, below |

## How it works (done)

`gui/app.c` is platform independent: it talks to the outside through a `host_t` (native EDIT fields, calls into the core, timers, quit) and draws through `w2k_gfx` (libw2k). Windows
supplies those in `gui/win_main.c` (GDI, real window); Linux supplies them in `gui/preview/preview_main.c` (libw2k's `w2k_png` software canvas that writes PNG, a synchronous call into the
**real Go core** built as a native c-archive, fixture machines in a temp directory). So the same application code runs on both. `scripts/test-gui.sh` builds that Linux
host and runs:

1. **Scripted UI journey** (`host --selftest`): the app's own self-test state machine (in `app.c`, also available on Windows as `-selftest <dir>`) acts like a user:
   list, case-insensitive search, every language, copy m1 FM1 -> FM2 (the plan must show the red deletion of the old `M1`), job to completion and file verification through the
   core, group rename m2 -> r2, refused rename ("bad/name"), the Excel entry. Exit code 0 = passed.
2. **Snapshot comparison** (`host --check tests/snapshots`): renders 14 screens (main window in ko/zh-Hans/zh-Hant/en; plan, job, menu, combo, refusal in ko and en) at a fixed
   14 px font with DejaVu Sans and Noto Sans CJK and compares with the committed baselines `tests/snapshots/*.png`. A pixel counts as different when a channel differs by more
   than 24; a screen fails above 0.30 % different pixels (tolerates anti-aliasing differences between font builds, catches any colour/layout change: changing the face colour fails at 41 %).
   On failure the rendered images and `*.diff.png` (red = different) are copied to `shots/failed/` (uploaded as a CI artifact).
3. After a deliberate look change: `scripts/test-gui.sh --update`, review the PNGs, commit them. `--render DIR` only renders.

`scripts/build.sh` runs it after the Go tests; CI job `gui-linux` runs it on Ubuntu (needs `gcc zlib1g-dev python3 fonts-dejavu-core fonts-noto-cjk`; the work path
`/tmp/afvi-gui-test` is fixed because the status bar shows it).

What it proves: layout, colours, bevels, text clipping and string lengths in four languages, every action of the app against the real core. What it does not: GDI specifics
(font metrics, ClearType, DPI), real window messages, native EDIT/IME, frameless caption.

## Still to do

- Control-level snapshot tests (one image per control state) once the controls are extracted (`docs/COMPONENT_LIBRARY.md`).
- Keyboard and mouse-event tests through `app_key` / `app_mouse` (list navigation, Tab order once it exists).
- A Windows run of `-selftest` in CI (`smoke-windows`).

## 4. Wine (optional)

User-level Wine (`apt download` + `dpkg -x`, bubblewrap) started its server and prefix, but its `user32.dll` was a stub (`CreateDialogParamW` unimplemented), so
no program ran. A full Wine from the distribution package (root or a container) is the closest thing to running the real exe on Linux (GDI text, messages, the
`-selftest`); try it where root or Docker is available. Wine's default look is Windows 2000-like, which does not prove the Windows 10/11 look.

## What Linux cannot tell you (needs a half-day Windows smoke test)

Start-up, GDI font fallback (Tahoma, Malgun Gothic, YaHei, JhengHei), the EDIT controls' look and Korean/Chinese IME, frameless caption on Windows 10/11 (corners,
maximise over the taskbar), DPI changes, idle memory, antivirus behaviour for a Go+cgo exe. CI placeholder `smoke-windows` in `.github/workflows/ci.yml` is where the
automated version goes (`windows-latest`, run `AfviTools.exe -selftest <dir>`).
