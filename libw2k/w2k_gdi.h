/* libw2k backend for Windows: a w2k_gfx that draws with GDI into a device context (typically a memory DC for double buffering).
 * This is the only libw2k file that includes <windows.h>. */
#ifndef W2K_GDI_H
#define W2K_GDI_H
#include <windows.h>
#include "w2k_gfx.h"

/* (Re)creates the regular and bold fonts and measures the line height with `dc`. nonaliased requests classic non-anti-aliased
 * text (used for Tahoma; CJK faces stay smoothed). Returns the text height. */
int  w2k_gdi_set_font(HDC dc, const wchar_t *face, int px, int nonaliased);
HFONT w2k_gdi_font(int bold);                 /* e.g. for WM_SETFONT of native EDIT controls */
w2k_gfx w2k_gdi_gfx(HDC dc);                  /* a graphics context drawing into dc with the current fonts */
#endif
