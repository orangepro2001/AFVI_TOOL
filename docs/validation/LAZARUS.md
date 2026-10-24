# Validation: Lazarus LCL from Go (branch `lazarus-lcl`)

Question: can the tool be a Lazarus (LCL) application driven from Go (`github.com/energye/golcl` v1.1.2, the maintained fork of GoVCL, plus the prebuilt
`liblcl` native library v2.5.4), in-process with the Go service layer, and can it show the **Windows 2000 look on Windows/Linux and a Mac OS 8/9
"Platinum" look on macOS** through custom drawing?

Date 2026-10-24, Linux dev machine, GTK3 widgetset, Xvfb. **Nothing was run on Windows or macOS** (no Windows, no Wine, no Mac).

## What was built (`core/cmd/afvi-lcl`)

Functional parity with the Wails and Avalonia GUIs, calling `service.Service` in process (no HTTP): menus (File, View > language / font size, Tools, Help), two pages,
source group (machine, node, search, refresh), report list with column sorting and a case-insensitive filter, copy group (single / whole group, optional new name),
rename group (single / whole group), rules, four-pane status bar, **modal plan dialog** (steps, danger steps in red, warnings, blockers disable Run), **modal job
dialog** (progress bar, counts, log, close), message boxes, four languages with live switching (ko default, same JSON packs, `_` access keys mapped to LCL `&`),
font size menu. Machine dialog: list + "restore default AFVI 13" only (add/edit not ported).

Three skins, `-skin=win2000|platinum|native` (default: platinum on macOS, win2000 elsewhere):

| Skin | What is drawn by us (`skin.go`, `widgets.go`) | What stays the widgetset's |
| --- | --- | --- |
| `native` | nothing | everything |
| `win2000` | caption bar (gradient, min/max/close, drag; the form is borderless), menu bar titles, tab strip, push buttons (raised/pressed/disabled/focus), group boxes (etched), status bar panes + grip, panel frames | edit, combo, list view + header, radio/check box, scroll bars, **drop-down menus (popup menus)**, message boxes' frame |
| `platinum` | same set with Platinum drawing: pinstriped title with close box left and zoom boxes right, rounded gradient buttons with a double outline for the default one, rounded tabs, thin well-style group frames | same list as above |

The Platinum drawing is **from memory of Mac OS 8/9** (face #DDDDDD, white/#888 bevels, black outlines, pinstripes); the highlight colour is a guess. It needs a
reference screenshot before anyone calls it authentic.

Tests: `go test ./...` (language packs complete with equal placeholders, every key used in the source exists, default language Korean, BGR colour order,
skins differ) and **`-selftest=<dir>`**: the real window is driven through its own handlers by a timer state machine (list, search, every language, no BM target
for FM, copy with the red deletion step shown in the modal plan dialog and the modal job dialog, group rename, refused plan, Excel entry) and the files are
checked with `wailsapp.SelftestVerify`. It passes for all three skins (`scripts/lcl.sh`).

## Measurements (Linux; baselines from the other branches)

| | Lazarus + golcl | Wails + RPC | Avalonia (trimmed single file) |
| --- | --- | --- | --- |
| Windows files | `afvi-lcl.exe` 10.4 MB + `liblcl.dll` 8.2 MB = **18.6 MB**, two files (liblcl can be embedded and released to a user folder by `inits.Init`; not exercised) | 12 MB, one file | 24 MB, one file |
| Windows exe built from Linux | yes, **pure Go, no cgo** (`GOOS=windows`); `-tags winstyles` adds the visual-styles manifest (+0.1 MB) | yes | yes |
| Linux | 10.4 MB + `liblcl.so` (GTK3) 12 MB = 22.4 MB | 10 MB (needs webkit2gtk) | 25 MB |
| macOS | **cannot be built from Linux**: golcl uses cgo there (`clang -arch` unsupported, `GetProcAddr` missing without it); `liblcl.dylib` is 9.4 MB. Needs a Mac host or CI runner | needs a Mac host | cross-built 26 MB |
| Idle RSS (Linux, Xvfb) | **155-163 MB** | ~460 MB | ~208 MB |
| Window on screen after launch | **0.11-0.15 s** | ~0.25 s | ~1.2 s |
| Hand-written GUI code | about 2,700 lines Go (incl. two skins, tests) | about 460 lines JS/HTML/CSS | about 1,960 lines C#/XAML |

## Hypothesis 1: "with modern styles off, Windows falls back to the Windows 2000 look"

**Partly true, not enough for this look; not verified here.**
- Without the comctl32 v6 manifest (golcl's `winappres` import, left out by default; `-tags winstyles` adds it) the Win32 controls are drawn in the classic
  style (grey 3D bevels). That part is real Windows behaviour and needs no code.
- But the colours come from the *current system colours*. On Windows 10/11 the face is #F0F0F0 and the 3D colours differ from the Windows 2000 set
  (#D4D0C8, #808080, #404040), so it looks like Windows 95 on a modern grey, not like Windows 2000. Setting `Color` on each control fixes the face only.
- Caption bar, drop-down menus, menu bar and the window frame (rounded corners on Windows 11) stay modern when the OS draws them. Here the caption bar, the menu bar
  and the frame are drawn by us (borderless form), which is what actually gives the look; the **drop-down menus are still native popups** (a custom drop-down needs
  an owner-drawn or hand-made window).
- Conclusion: classic controls are a *starting point* (they remove the Windows 10 flat style for free) but the look is only right where we draw it. Check on Windows 10 and 11.

## Hypothesis 2: "with custom drawing the same app shows Platinum on macOS"

**Feasible for the custom-drawn set, expensive for the rest; not verified here.**
- The skin code only uses `TCanvas` (rectangles, lines, round rectangles, text), so it behaves the same on GTK3, Win32 and Cocoa in principle.
  On macOS the default skin is `platinum` and the same borderless-form + painted caption/menu/tab code applies.
- Everything we do not draw stays Aqua there: edit, combo, list, check/radio boxes, scroll bars, popup menus. A coherent Platinum look needs custom replacements
  for them (about as much work again as the skin set done here, plus keyboard, focus and IME behaviour that native controls give for free).
- macOS cannot be built or tested from this machine at all (cgo + a Mac SDK).

## What did not work or surprised (GTK3, golcl 1.1.2)

- `TListView.OwnerDraw` + `OnDrawItem` is **not honoured on GTK3**: rows keep the GTK selection colour (orange in the screenshots) and header stays native.
  Colouring the list needs another approach (custom-drawn list in a `TPaintBox`).
- `TTabSheet.SetColor` crashes (nil); tab sheets whose tab is hidden hide their page too: the skinned build uses two plain panels and its own tab strip.
- `Align=Top` stacks controls in reverse creation order; the form size set in `OnFormCreate` is overwritten when the window appears (set from a timer after show).
- Setting `Checked` on radio menu items fires their `OnClick`: language switching looped until guarded.
- GTK prints many `Gtk-CRITICAL` lines for custom controls (harmless in the runs here, noisy).
- `lcl.DEBUG=true` is needed to see a panic's stack in an event handler (otherwise golcl swallows it and prints one line).
- Font size is in points and the layout is fixed pixels: at 14 pt the right column truncates (screenshots are written to `shots/`, not committed). Scaling would need a layout pass (anchors or a sizer-style layout).
- Korean/Chinese IME was not tested (GTK3 input method needs a session; the Win32 and Cocoa widgetsets use the system IME).

## Risks and licensing

- `liblcl` is a **binary built by a third party** (energye/liblcl; last release 2025-06, v2.5.4; no signature, no reproducible build here). Building it needs Lazarus + FPC
  (not installed, not attempted). golcl itself is Apache-2.0; LCL is modified LGPL with a static-linking exception; liblcl is linked dynamically.
- The community is small (golcl ~8 stars on the fork, ying32/govcl 2.4k stars but GTK2-only releases, last 2023). Expect to read source for every missing feature.
- Linux needs GTK3 at run time; the older govcl releases need GTK2, which this machine does not have.

## Verdict: go with conditions, not the default

Best start-up (0.1 s), small memory (about 160 MB) and a mid-size download (18.6 MB on Windows, two files), a pure-Go cross-build for Windows, and one set of drawing
code for both looks. But the look exists only where we draw it, the native controls are not themed (edit/combo/list/menus), macOS cannot be built from Linux, and the
native library is a third-party binary. Choose it only if the **Platinum-on-macOS requirement is real** and you accept replacing the remaining native controls; for a Windows 2000 look
alone the Wails or Avalonia builds are less work.

Effort to production grade: about 2-3 weeks (custom list, edit, combo, check/radio, popup menus and scroll bars in both skins; layout that scales with the font;
macOS build and Platinum reference comparison; IME checks on three systems; own liblcl build).

Build and test: `scripts/lcl.sh` (downloads liblcl v2.5.4, needs `xvfb-run` for the self-test).
