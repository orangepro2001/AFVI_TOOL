/* The few drawing operations the Windows 2000 skin needs. Backends: GDI (Windows) and a PNG software
 * canvas (Linux preview). Colours are 0xRRGGBB, coordinates whole pixels. */
#ifndef GFX_H
#define GFX_H
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
} gfx_t;
#endif
