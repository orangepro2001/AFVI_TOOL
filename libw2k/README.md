# libw2k: Windows 2000 controls in C

Owner-drawn Windows 2000 look (tokens of the `win2000-style` skill) for a plain Win32 or any other host. C99, no dependencies except the backends.
Extracted from AFVI TOOLS with no change of look: the 14 application snapshots are unchanged.

| Layer | Files | Contains |
| --- | --- | --- |
| 0 drawing interface | `w2k_gfx.h` | `w2k_gfx`: fill, gradient, text, text width, clip (five function pointers + text height) |
| 1 tokens and primitives | `w2k.h`, `w2k_draw.c` | colours, six bevel recipes, button/check/tab/group/field/progress/scrollbar/caption primitives, `w2k_font_px`, `w2k_px()` |
| 2 controls | `w2k.h`, `w2k_ctl.c` | button, check/radio, combo + drop list, property sheet, menu bar, popup menu, report list, text view, status bar, dialog frame, window caption; hit regions in a `w2k_hotlist` |
| backend Windows | `w2k_gdi.h/.c` | GDI font and `w2k_gfx` on an HDC. The only file that includes `<windows.h>` |
| backend Linux | `w2k_png.h/.c`, `third_party/stb_truetype.h` | software canvas, PNG writer/reader (zlib), tolerant snapshot comparison |

Layers 0 to 2 never include `<windows.h>` (checked by `grep windows.h libw2k/*`): that is what lets the same code render on Linux.

## Using it

```c
w2k_gfx g = w2k_gdi_gfx(hdc);                 /* or w2k_png_gfx() on Linux */
w2k_hot_reset(&hot);
w2k_button(&g, &hot, x, y, w, h, L"OK", ACT_OK, 0, W2K_DEFAULT);
...
int i = w2k_hot_at(&hot, mouse_x, mouse_y);   /* on click: hot.v[i].act / .arg */
```

The library keeps no state: the application owns selection, scroll offsets and open menus, and passes them in each paint. Clicks are resolved with the
hit-region list the controls fill while painting (later regions win, so modal layers register last).

## Tests and demo

`libw2k/tests/run.sh` builds `demo/demo.c` (a gallery of every control, no application code, no Go) and runs the logic checks (hit list, list scrolling, scaling)
plus a snapshot comparison with `libw2k/tests/snapshots` (14 px, 14 px with menu and drop list open, 18 px). `--update` rewrites the baselines after a deliberate
look change. Needs gcc, zlib headers and `fonts-dejavu-core`. Tahoma is not on Linux, so the images use DejaVu Sans.

## Not extracted (stays in `gui/app.c`)

- Layout and flow of the application's screens, the plan/job dialogs' contents, the Excel entry, all state and `host_t` (native EDIT fields, timers, core calls).
- Keyboard handling and focus order: it is bound to the application's actions; the library only exposes hit regions.
- Text editing (the app uses native EDIT children for IME); a library edit control would need its own caret, selection and IME code.
- Dark theme, DPI scaling beyond `w2k_font_px`, tree, grid editing, date picker: not needed yet (see `docs/COMPONENT_LIBRARY.md`).
- Licence and a separate repository: undecided; `third_party/stb_truetype.h` is public domain / MIT (Linux backend only).
