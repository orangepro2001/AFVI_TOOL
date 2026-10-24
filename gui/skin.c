/* Windows 2000 skin primitives: five system colours, 2 px bevels, no rounding, no hover colour. */
#include "skin.h"
#include <wchar.h>

int skin_font_px = 14;

static void rect(gfx_t *g, int x, int y, int w, int h, uint32_t c) { if (w > 0 && h > 0) g->fill(g->ctx, x, y, w, h, c); }
/* one 1 px ring: top and bottom strips own the corners, as in the reference */
static void ring(gfx_t *g, int x, int y, int w, int h, uint32_t tl, uint32_t br) {
    rect(g, x, y, w - 1, 1, tl);
    rect(g, x, y + 1, 1, h - 2, tl);
    rect(g, x, y + h - 1, w, 1, br);
    rect(g, x + w - 1, y, 1, h - 1, br);
    rect(g, x + w - 1, y, 1, 1, br);
}

void skin_edge(gfx_t *g, int x, int y, int w, int h, edge_t k) {
    switch (k) {
    case SKE_RAISED:  ring(g, x, y, w, h, C_HIGHLIGHT, C_DKSHADOW); ring(g, x + 1, y + 1, w - 2, h - 2, C_LIGHT, C_SHADOW); break;
    case SKE_SUNKEN:  ring(g, x, y, w, h, C_SHADOW, C_HIGHLIGHT);   ring(g, x + 1, y + 1, w - 2, h - 2, C_DKSHADOW, C_LIGHT); break;
    case SKE_PRESSED: ring(g, x, y, w, h, C_DKSHADOW, C_DKSHADOW);  ring(g, x + 1, y + 1, w - 2, h - 2, C_SHADOW, C_SHADOW); break;
    case SKE_STATUS:  ring(g, x, y, w, h, C_SHADOW, C_HIGHLIGHT); break;
    case SKE_ETCHED:  ring(g, x + 1, y + 1, w - 1, h - 1, C_HIGHLIGHT, C_HIGHLIGHT); ring(g, x, y, w - 1, h - 1, C_SHADOW, C_SHADOW); break;
    case SKE_WINDOW:  ring(g, x, y, w, h, C_LIGHT, C_DKSHADOW);     ring(g, x + 1, y + 1, w - 2, h - 2, C_HIGHLIGHT, C_SHADOW); break;
    }
}

void skin_dotted(gfx_t *g, int x, int y, int w, int h) {
    for (int i = 0; i < w; i += 2) { rect(g, x + i, y, 1, 1, C_TEXT); rect(g, x + i, y + h - 1, 1, 1, C_TEXT); }
    for (int j = 0; j < h; j += 2) { rect(g, x, y + j, 1, 1, C_TEXT); rect(g, x + w - 1, y + j, 1, 1, C_TEXT); }
}

void skin_text_disabled(gfx_t *g, int x, int y, const wchar_t *s, int n) {
    g->text(g->ctx, x + 1, y + 1, s, n, C_HIGHLIGHT, 0);   /* embossed: Highlight at +1,+1 under Shadow */
    g->text(g->ctx, x, y, s, n, C_SHADOW, 0);
}

void skin_caption(gfx_t *g, int x, int y, int w, int h, const wchar_t *title, int active) {
    if (active) g->gradient(g->ctx, x, y, w, h, C_CAPTION_L, C_CAPTION_R);
    else        g->gradient(g->ctx, x, y, w, h, C_SHADOW, C_FACE);
    int ty = y + (h - g->text_h) / 2, n = (int)wcslen(title);
    g->text(g->ctx, x + 4, ty, title, n, 0xFFFFFF, 1);
    g->text(g->ctx, x + 5, ty, title, n, 0xFFFFFF, 1);      /* bold faked by drawing twice 1 px apart */
}

void skin_caption_button(gfx_t *g, int x, int y, int w, int h, int glyph, int pressed) {
    rect(g, x, y, w, h, C_FACE);
    skin_edge(g, x, y, w, h, pressed ? SKE_PRESSED : SKE_RAISED);
    int o = pressed, cx = x + w / 2 + o, cy = y + h / 2 + o, s = h / 5 > 2 ? h / 5 : 2;
    if (glyph == 0) rect(g, cx - s, cy + s, 2 * s, 2, C_TEXT);                       /* minimise */
    else if (glyph == 1) { skin_edge(g, cx - s - 1, cy - s - 1, 2 * s + 2, 2 * s + 2, SKE_STATUS); rect(g, cx - s - 1, cy - s - 1, 2 * s + 2, 2, C_TEXT); }
    else for (int i = -s; i <= s; i++) { rect(g, cx + i, cy + i, 2, 1, C_TEXT); rect(g, cx + i, cy - i, 2, 1, C_TEXT); } /* close */
}

void skin_button(gfx_t *g, int x, int y, int w, int h, const wchar_t *label, int pressed, int disabled, int is_default, int focus) {
    rect(g, x, y, w, h, C_FACE);
    if (is_default && !pressed) { ring(g, x, y, w, h, C_TEXT, C_TEXT); x++; y++; w -= 2; h -= 2; rect(g, x, y, w, h, C_FACE); }
    skin_edge(g, x, y, w, h, pressed ? SKE_PRESSED : SKE_RAISED);
    int n = (int)wcslen(label), tw = g->text_w(g->ctx, label, n, 0);
    int tx = x + (w - tw) / 2 + (pressed ? 1 : 0), ty = y + (h - g->text_h) / 2 + (pressed ? 1 : 0);
    if (disabled) skin_text_disabled(g, tx, ty, label, n); else g->text(g->ctx, tx, ty, label, n, C_TEXT, 0);
    if (focus && !disabled) skin_dotted(g, x + 4, y + 4, w - 8, h - 8);
}

void skin_group(gfx_t *g, int x, int y, int w, int h, const wchar_t *title) {
    int half = g->text_h / 2;
    skin_edge(g, x, y + half, w, h - half, SKE_ETCHED);       /* interior is never filled */
    int n = (int)wcslen(title), tw = g->text_w(g->ctx, title, n, 0);
    rect(g, x + pxs(8), y, tw + 6, g->text_h, C_FACE);          /* Face gap behind the title, title at x+10 */
    g->text(g->ctx, x + pxs(8) + 3, y, title, n, C_TEXT, 0);
}

void skin_check(gfx_t *g, int x, int y, int checked, int disabled, int radio) {
    int s = pxs(13);
    if (radio) {                                   /* round: sunken ring (Shadow/DkShadow top-left, Highlight/Light bottom-right) */
        int R2 = s;                                /* radius*2 */
        for (int j = 0; j < s; j++) for (int i = 0; i < s; i++) {
            int dx = 2 * i + 1 - s, dy = 2 * j + 1 - s, d2 = dx * dx + dy * dy;      /* distance*2, squared */
            if (d2 > R2 * R2) continue;
            uint32_t c;
            if (d2 > (R2 - 2) * (R2 - 2)) c = dx + dy < 0 ? C_SHADOW : C_HIGHLIGHT;
            else if (d2 > (R2 - 4) * (R2 - 4)) c = dx + dy < 0 ? C_DKSHADOW : C_LIGHT;
            else c = disabled ? C_FACE : C_FIELD;
            rect(g, x + i, y + j, 1, 1, c);
        }
        if (checked) { int r = s / 5, cx = x + s / 2, cy = y + s / 2; for (int j = -r; j <= r; j++) for (int i = -r; i <= r; i++) if (i * i + j * j <= r * r + 1) rect(g, cx + i, cy + j, 1, 1, disabled ? C_SHADOW : C_TEXT); }
        return;
    }
    rect(g, x, y, s, s, disabled ? C_FACE : C_FIELD);
    skin_edge(g, x, y, s, s, SKE_SUNKEN);
    if (!checked) return;
    uint32_t c = disabled ? C_SHADOW : C_TEXT;
    for (int i = 0; i < s / 3; i++) { rect(g, x + 3 + i, y + s / 2 + i - 1, 1, 3, c); rect(g, x + 3 + s / 3 + i, y + s / 2 + s / 3 - i - 2, 1, 3, c); }
}

void skin_tab(gfx_t *g, int x, int y, int w, int h, const wchar_t *label, int selected) {
    if (selected) { x -= 2; y -= 2; w += 4; h += 3; }
    rect(g, x, y, w, h, C_FACE);
    rect(g, x, y, 1, h, C_HIGHLIGHT); rect(g, x, y, w, 1, C_HIGHLIGHT);                     /* outer top/left */
    rect(g, x + 1, y + 1, 1, h - 1, C_LIGHT); rect(g, x + 1, y + 1, w - 2, 1, C_LIGHT);     /* inner top/left */
    rect(g, x + w - 1, y, 1, h, C_DKSHADOW); rect(g, x + w - 2, y + 1, 1, h - 1, C_SHADOW); /* right */
    int n = (int)wcslen(label), tw = g->text_w(g->ctx, label, n, 0);
    g->text(g->ctx, x + (w - tw) / 2, y + (h - g->text_h) / 2 + (selected ? 0 : 1), label, n, C_TEXT, 0);
}

void skin_pane(gfx_t *g, int x, int y, int w, int h, const wchar_t *text, int clip) {
    skin_edge(g, x, y, w, h, SKE_STATUS);
    if (clip) g->clip(g->ctx, x + 2, y, w - 4, h);
    g->text(g->ctx, x + 6, y + (h - g->text_h) / 2, text, (int)wcslen(text), C_TEXT, 0);
    if (clip) g->clip(g->ctx, 0, 0, 0, 0);
}

void skin_field(gfx_t *g, int x, int y, int w, int h) { rect(g, x, y, w, h, C_FIELD); skin_edge(g, x, y, w, h, SKE_SUNKEN); }

void skin_combo(gfx_t *g, int x, int y, int w, int h, const wchar_t *text, int disabled) {
    skin_field(g, x, y, w, h);
    int bw = h - 4;                                                    /* the arrow button is a small raised button */
    g->clip(g->ctx, x + 3, y + 2, w - bw - 6, h - 4);
    if (disabled) skin_text_disabled(g, x + 4, y + (h - g->text_h) / 2, text, (int)wcslen(text));
    else g->text(g->ctx, x + 4, y + (h - g->text_h) / 2, text, (int)wcslen(text), C_TEXT, 0);
    g->clip(g->ctx, 0, 0, 0, 0);
    rect(g, x + w - bw - 2, y + 2, bw, h - 4, C_FACE);
    skin_edge(g, x + w - bw - 2, y + 2, bw, h - 4, SKE_RAISED);
    int cx = x + w - bw / 2 - 2, cy = y + h / 2 - 1;                   /* solid triangle */
    for (int i = 0; i < 4; i++) rect(g, cx - 3 + i, cy + i - 1, 7 - 2 * i, 1, disabled ? C_SHADOW : C_TEXT);
}

void skin_progress(gfx_t *g, int x, int y, int w, int h, int pct) {
    skin_field(g, x, y, w, h);
    int inner = w - 6, bw = pxs(8), gap = 2, n = (inner * (pct < 0 ? 0 : pct > 100 ? 100 : pct)) / 100;
    for (int px = 0; px + bw <= n; px += bw + gap) rect(g, x + 3 + px, y + 3, bw, h - 6, C_SELECT);   /* blocks, not a smooth bar */
}

void skin_header_cell(gfx_t *g, int x, int y, int w, int h, const wchar_t *text) {
    rect(g, x, y, w, h, C_FACE);
    skin_edge(g, x, y, w, h, SKE_RAISED);
    g->clip(g->ctx, x + 2, y, w - 4, h);
    g->text(g->ctx, x + 6, y + (h - g->text_h) / 2, text, (int)wcslen(text), C_TEXT, 0);
    g->clip(g->ctx, 0, 0, 0, 0);
}

void skin_scrollbar(gfx_t *g, int x, int y, int w, int h, int pos, int page, int total) {
    rect(g, x, y, w, h, C_LIGHT);
    int bs = w;                                                        /* square arrow buttons */
    rect(g, x, y, w, bs, C_FACE); skin_edge(g, x, y, w, bs, SKE_RAISED);
    rect(g, x, y + h - bs, w, bs, C_FACE); skin_edge(g, x, y + h - bs, w, bs, SKE_RAISED);
    for (int i = 0; i < 3; i++) { rect(g, x + w / 2 - i, y + bs / 2 - 1 + i, 2 * i + 1, 1, C_TEXT); rect(g, x + w / 2 - 2 + i, y + h - bs / 2 - 1 - i + 1, 5 - 2 * i, 1, C_TEXT); }
    int track = h - 2 * bs;
    if (total > page && track > 8) {
        int th = track * page / total; if (th < 12) th = 12;
        int ty = y + bs + (track - th) * pos / (total - page);
        rect(g, x, ty, w, th, C_FACE); skin_edge(g, x, ty, w, th, SKE_RAISED);
    }
}
