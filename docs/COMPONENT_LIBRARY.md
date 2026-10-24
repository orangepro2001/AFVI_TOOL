# Extracting the Windows 2000 controls into a component library

Status: **steps 1 to 3 done (2026-10-24): `libw2k/`**, see `libw2k/README.md` for what is in it and what stayed in the application. The text below is the original plan; the layers are named as built (`w2k_draw.c`, `w2k_ctl.c`, `w2k_gdi.c`, `w2k_png.c`). Why: the drawn controls (`gui/skin.c`, parts of `gui/app.c`, about 1,100 lines) are the part of this project that other tools
of the same family (for example the future Export Data To Excel screens) would reuse, and they are the part that is expensive to rewrite.

## Existing open source (checked 2026-10-24, only by search)

No library was found that reproduces the Windows 2000 look with owner-drawn Win32 controls. Found: DirectUI frameworks that draw all controls themselves with
XML layouts (DuiLib, NIM Duilib, XUI); they are skin engines, heavier to adopt than our own drawing code, and their Win2000 skin would still have to be written.
Native controls with visual styles off (`SetWindowTheme(hwnd, L"", L"")`, no comctl32 v6 manifest) go classic but take the *system* colours (Face #F0F0F0 on Windows 10, not
#D4D0C8) and a modern caption bar, so they do not give this look. Decision: build our own, keep it small.

## Layers

1. **Tokens and primitives** (`w2k_tokens.h`, `w2k_draw.c`): palette (light, optional dark), the six bevel recipes (raised, sunken, pressed, status, etched, window),
   focus rectangle, embossed disabled text, gradient caption. Depends only on `gfx_t` (`fill`, `gradient`, `text`, `text_w`, `clip`).
2. **Controls** (`w2k_button.c`, `w2k_check.c`, `w2k_radio.c`, `w2k_group.c`, `w2k_tabs.c`, `w2k_list.c`, `w2k_combo.c`, `w2k_progress.c`, `w2k_status.c`, `w2k_menu.c`,
   `w2k_dialog.c`): each is a small struct (rect, state, text, callbacks) with `paint(gfx, ...)`, `hit_test`, `on_mouse`, `on_key`. No Win32 calls inside.
3. **Win32 host** (`w2k_win32.c`): one window class that owns the controls, translates WM_PAINT/WM_MOUSE*/WM_KEY*/WM_CHAR into control events, provides the GDI `gfx_t`,
   hosts native EDIT children for text entry (IME), optional frameless caption (WM_NCCALCSIZE, WM_NCHITTEST), DPI/font scaling.
4. **Preview/test host** (`w2k_png.c`): the software `gfx_t` that writes PNG, used by snapshot tests on Linux.

Rule: layers 1 and 2 never include `<windows.h>`; that is what makes Linux testing possible (see `docs/LINUX_TESTING.md`).

## Steps

1. Move `skin.c/h`, `gfx.h` into `libw2k/` with the layer split above; keep `app.c` compiling against it (no behaviour change; the preview PNGs must stay identical).
2. Pull the controls out of `app.c` one at a time (button, group, tabs, list, combo, progress, status, menu, dialog frame); each gets a snapshot test per state
   (normal, pressed, disabled, focused) x theme x font size.
3. Give the library its own `README`, version, licence (decide: this repository's, or a separate repository) and a demo program (the `demo.html` of the CSS port is the model:
   every control on one page, rendered to PNG).
4. Add the missing controls the application did not need yet when they are needed (tree, editable grid, date picker); do not build them in advance.
5. Keep the design tokens in sync with `~/.claude/skills/win2000-style` (the specification); when they disagree the tokens win.

## Effort and open decisions

About 2 to 3 days for steps 1 to 3. Decide: separate repository or `libw2k/` here; licence; whether to offer a C++ wrapper; whether to include the dark theme.
