#include "w2k_gdi.h"

static HFONT g_font[2];
static int g_text_h = 16;

static COLORREF cr(uint32_t rgb) { return RGB((rgb >> 16) & 255, (rgb >> 8) & 255, rgb & 255); }
static void g_fill(void *c, int x, int y, int w, int h, uint32_t rgb) { RECT r = {x, y, x + w, y + h}; HBRUSH b = CreateSolidBrush(cr(rgb)); FillRect((HDC)c, &r, b); DeleteObject(b); }
static void g_gradient(void *c, int x, int y, int w, int h, uint32_t l, uint32_t r) {
    for (int i = 0; i < w; i++) {
        int rr = (int)((l >> 16) & 255) + ((int)((r >> 16) & 255) - (int)((l >> 16) & 255)) * i / (w > 1 ? w - 1 : 1);
        int gg = (int)((l >> 8) & 255) + ((int)((r >> 8) & 255) - (int)((l >> 8) & 255)) * i / (w > 1 ? w - 1 : 1);
        int bb = (int)(l & 255) + ((int)(r & 255) - (int)(l & 255)) * i / (w > 1 ? w - 1 : 1);
        g_fill(c, x + i, y, 1, h, ((uint32_t)rr << 16) | ((uint32_t)gg << 8) | (uint32_t)bb);
    }
}
static void g_text(void *c, int x, int y, const wchar_t *s, int n, uint32_t rgb, int bold) {
    HDC dc = (HDC)c; SelectObject(dc, g_font[bold ? 1 : 0]); SetBkMode(dc, TRANSPARENT); SetTextColor(dc, cr(rgb)); ExtTextOutW(dc, x, y, 0, NULL, s, n, NULL);
}
static int g_text_w(void *c, const wchar_t *s, int n, int bold) { SIZE sz; SelectObject((HDC)c, g_font[bold ? 1 : 0]); GetTextExtentPoint32W((HDC)c, s, n, &sz); return sz.cx; }
static void g_clip(void *c, int x, int y, int w, int h) {
    if (w <= 0) { SelectClipRgn((HDC)c, NULL); return; }
    HRGN r = CreateRectRgn(x, y, x + w, y + h); SelectClipRgn((HDC)c, r); DeleteObject(r);
}

int w2k_gdi_set_font(HDC dc, const wchar_t *face, int px, int nonaliased) {
    for (int i = 0; i < 2; i++) if (g_font[i]) DeleteObject(g_font[i]);
    for (int i = 0; i < 2; i++)
        g_font[i] = CreateFontW(-px, 0, 0, 0, i ? FW_BOLD : FW_NORMAL, 0, 0, 0, DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS,
                                nonaliased ? NONANTIALIASED_QUALITY : DEFAULT_QUALITY, DEFAULT_PITCH, face);
    SelectObject(dc, g_font[0]); TEXTMETRICW tm; GetTextMetricsW(dc, &tm); g_text_h = tm.tmHeight;
    return g_text_h;
}
HFONT w2k_gdi_font(int bold) { return g_font[bold ? 1 : 0]; }
w2k_gfx w2k_gdi_gfx(HDC dc) { w2k_gfx g = {dc, g_fill, g_gradient, g_text, g_text_w, g_clip, g_text_h}; return g; }
