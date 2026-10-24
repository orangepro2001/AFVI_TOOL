# Validation: native Win32 GUI (C) + Go as a core DLL / static library (branch `win32-native`)

Date 2026-10-24, Linux dev machine, mingw-w64 GCC 13 (win32 threads), Go with cgo. Windows only, cross-built from Linux. **Nothing was run on
Windows**: there is no Windows machine here and Wine could not be made to work (see "Verification"). Parts that were run are marked.

## What was built

- `core/internal/capi` (plain Go, unit tested, **run**): the whole core behind one call `Call(method, lang, jsonArgs) -> {"ok":..,"result":..}|{"ok":false,"error":..}`;
  panics become errors. `core/cmd/afvi-core-dll` (cgo): `afvi_init`, `afvi_call`, `afvi_free` (header `gui/afvi_core.h`), built as a DLL (`-buildmode=c-shared`)
  and as a static archive (`c-archive`) with `x86_64-w64-mingw32-gcc`.
- `gui/` (C99, plain Win32 W API, GDI, cJSON vendored under MIT): one window; **everything is drawn by the application** (menus, tabs, buttons, check/radio boxes,
  combos, the report list, dialogs, status bar, optional caption bar) with the Windows 2000 tokens; the only native child windows are three EDIT controls, so Korean/Chinese
  IME input is the system's. Same functionality as the other front ends: machine/node combos, search, list with column sort, copy (single/group, new name), rename
  (single/group), plan-confirm dialog (red deletion steps, warnings, blockers disable Run), job dialog (progress, log, cancel before the point of no return), refusal
  messages, export entry, four languages with live switch (same JSON packs, embedded by `gen_lang.py`), font size menu. Jobs and planning run on worker threads.
  `-frameless` draws the Windows 2000 caption bar (WM_NCCALCSIZE + WM_NCHITTEST); `-selftest <dir>` drives the real UI with the same journey as the Wails/Avalonia
  self-tests and verifies the files through the core (not run, no Windows).
- `gui/preview`: the same `app.c` + `skin.c` rendered to PNG on Linux with a software canvas against the **real core** (native c-archive) on throw-away machines: copy
  plan, job, menus, combos, refusal, four languages. This is how the look was reviewed (`docs/validation/win32-preview/*.png`; `scripts/win32.sh` regenerates them).
  The preview uses DejaVu Sans / Noto Sans CJK because Tahoma is not on Linux (the language menu shows boxes in the English screenshots only because DejaVu has no
  Hangul; Windows font linking covers it).
- `scripts/win32.sh`: cross-builds all of it, both link modes, and prints sizes and imported DLLs.

## Results (measured on the build outputs)

| Build | Files | Size | Imports (besides the system DLLs below) |
| --- | --- | --- | --- |
| Dynamic link: `afvi-dyn.exe` + `afvi-core.dll` | 2 | exe 0.2 MB + dll 5.1 MB = **5.3 MB** | exe: afvi-core.dll; dll: kernel32, msvcrt |
| Static link: `afvi-static.exe` (core archive inside) | 1 | **5.2 MB** | gdi32, kernel32, msvcrt, shell32, user32 |
| `-static` variants | same as above | identical sizes | this mingw (win32 thread model) needs no libgcc/libstdc++/winpthread DLL either way |

Baselines (Windows): Wails 12 MB (1 file), Avalonia trimmed 24 MB (1 file), Lazarus 10.4 + 8.2 MB (2 files), Avalonia AOT 33 + 16 MB. This is the smallest,
and its only runtime dependencies are DLLs present in every Windows since NT4/98 (`msvcrt.dll`, `kernel32`, `user32`, `gdi32`, `shell32`); no .NET, no WebView2, no VC++
redistributable, no manifest dependency on comctl32 v6. The Go runtime is 5 MB of it; the C GUI is only about 0.2 MB. Idle memory and start time were not measured (no Windows).

## Dynamic vs static linking

- **Static** (c-archive into the exe): one file, no DLL search path issues, nothing to deploy next to it; 5.2 MB. Go starts its runtime inside the exe's process; the exe is
  built with `-s` (stripped). Recommended for this tool.
- **Dynamic** (c-shared): the GUI exe is 0.2 MB and can be replaced without touching the core, and the DLL could be used by another host (Excel add-in, another tool);
  costs: two files that must stay together, DLL search order (put both in one folder; use `SetDllDirectory`/`LoadLibraryEx` flags if installed in a shared path),
  and a Go runtime that lives in a DLL (cannot be unloaded: never call `FreeLibrary`).
- Both link only to the old `msvcrt.dll`; a UCRT build (`-lucrt`/`-D_UCRT` toolchain) would add an `api-ms-win-crt-*` dependency (present on Windows 10, needs an update on 7/8).

## The Windows 2000 look and the Windows 10/11 caveats

- Skin primitives (`gui/skin.c`) implement the six bevel recipes, caption, buttons (default button, pressed offset, disabled embossed text, dotted focus), group box
  (etched, title gap), round radio, check box, tabs, combo, progress blocks, status panes, scrollbar, header cells with the exact tokens (Face #D4D0C8 ...). Nothing uses system
  colours, so the result does not depend on the Windows version or theme. No hover colour, no rounding (radio buttons are round, as in every Windows).
- No comctl32 v6 manifest: visual styles are off for the process, which is also what makes the three EDIT controls draw classic. The EDIT's own border is replaced by the skin's
  sunken field around it.
- **Title bar**: the default mode (`WS_OVERLAPPEDWINDOW`) shows the *system* caption: flat/modern on Windows 10/11 (gradient-less, Segoe UI, white or accent colour, rounded
  corners on 11). `-frameless` removes it and draws the gradient caption (drag, three buttons, edge resize via WM_NCHITTEST). Frameless caveats on Windows 10/11 that need a real
  check: the DWM shadow and 1 px border are gone (fine for this look) but Windows 11 may still round the corners of a `WS_POPUP|WS_THICKFRAME` window (opt out with
  `DwmSetWindowAttribute(DWMWA_WINDOW_CORNER_PREFERENCE, DWMWCP_DONOTROUND)`, not coded yet); snap layouts/maximise over the taskbar (`WM_GETMINMAXINFO` work-area clamp is not
  coded yet); the application menu is drawn by the app, so no native menu bar is involved.
- **Text**: Tahoma for English, otherwise Malgun Gothic / Microsoft YaHei / Microsoft JhengHei (each has Latin glyphs too). Classic non-anti-aliased Tahoma is requested with
  `NONANTIALIASED_QUALITY`; CJK uses the default quality (hard-edged CJK is unreadable at 14 px).
- **DPI**: `SetProcessDPIAware()` (system DPI aware); all lengths scale with the font px. Per-monitor DPI changes are not handled (needs WM_DPICHANGED and a manifest).
- **IME**: native EDIT controls: composition windows, candidate lists and Korean/Chinese input work as in any Win32 app. Keyboard handling in the drawn parts is minimal
  (arrows/page/home/end in the list, Enter/Esc in dialogs, F5); no Tab-order between drawn buttons, no screen-reader support (UI Automation would need a custom provider).

## Effort and risks

- Toolchain: mingw-w64 (apt) and Go cgo cross-compilation worked first time; the build is one script. The C code is about 1,900 lines (skin 190, app 930, host 190, i18n/util 120, preview 200).
  Everything drawn by hand costs time: every new control (a tree, a date picker, an editable grid) means writing and hit-testing it. That is the price of this look; the Wails
  version gets controls from the browser and Avalonia from its theme.
- Go runtime inside the exe: it starts threads at `afvi_init`; **a Go panic must never cross the C boundary**: handled in `capi.Call` (recover -> JSON error). Calls from several threads
  are safe (service state has locks). GC pauses are not an issue at this allocation rate. Do not call back into C from Go (not used). Memory returned by `afvi_call` is C-malloc'ed
  and must be released with `afvi_free` (done).
- Antivirus: a Go+cgo exe is a common false-positive trigger; static vs dynamic made no difference in principle.
- Maintenance: two languages (C, Go) and a hand-made UI layer; no UI test framework. The preview renderer plus the Go tests are the safety net; the self-test exists but was not run.

## Verification: what was and was not done

- Run on Linux: Go unit tests of `capi` (full journey through the C-ABI-shaped calls: list, plan, job, verify), cross-compilation of the DLL, the static archive and all four exes,
  `objdump -p` import lists, and the preview renderer (the actual application code and skin painting, real core, throw-away machines; screenshots reviewed and fixed: radio shape,
  list column overflow).
- Wine: the Ubuntu `wine64` + `libwine` + `wine-common` packages were downloaded and unpacked without root and run under bubblewrap (to fake `/usr/share/wine`); the Wine server and
  prefix start, but the loaded `user32.dll` is a stub ("unimplemented function user32.dll.CreateDialogParamW"), so no Windows program could run. Not pursued further.
- **Not verified**: starting the exe on Windows, GDI text rendering and font fallback, the EDIT controls' look and IME, frameless caption behaviour on 10/11, threads and the Go runtime inside
  the process, `-selftest`, idle memory and start time. The whole "looks like Windows 2000" claim rests on the preview renderer, which draws with the same code but not through GDI.

## Verdict: go, with conditions

Technically the lightest option and the most "traditional": 5 MB, no framework, only ancient system DLLs, Go confined to a clean C ABI that other hosts could also use. It reproduces the
Windows 2000 look exactly because it draws everything itself. Conditions: (1) a half-day smoke test on real Windows 7/10/11 (start-up, fonts, frameless caption, IME, the self-test);
(2) accept hand-writing UI controls (about 2-3 days to production grade: keyboard focus ring and Tab order, Win11 corner/maximise handling, DPI change, accessibility if needed);
(3) static link as the default delivery. If hand-drawn controls and C maintenance are not acceptable, Wails (12 MB) or Avalonia (24 MB) remain the lower-effort choices.
