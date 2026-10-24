/* libw2k layer 0: the few drawing operations the Windows 2000 skin needs. Backends: GDI (w2k_gdi.c, Windows) and a PNG
 * software canvas (w2k_png.c, Linux tests and previews). Colours are 0xRRGGBB, coordinates whole pixels. */
#ifndef W2K_GFX_H
#define W2K_GFX_H
#include <stdint.h>
#include <wchar.h>
typedef struct gfx {
    void *ctx;
    void (*fill)(void *ctx, int x, int y, int w, int h, uint32_t rgb);
    void (*gradient)(void *ctx, int x, int y, int w, int h, uint32_t left, uint32_t right);
    void (*text)(void *ctx, int x, int y, const wchar_t *s, int n, uint32_t rgb, int bold); /* top-left, transparent */
    int  (*text_w)(void *ctx, const wchar_t *s, int n, int bold);
    void (*clip)(void *ctx, int x, int y, int w, int h);   /* w<=0: remove clip */
    int  text_h;                                            /* line height of the current font */
} w2k_gfx;
#endif
