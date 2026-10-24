/* libw2k layer 2: controls. Each one paints with the layer 1 primitives through w2k_gfx and appends its hit regions to a
 * w2k_hotlist; the application keeps the state (checked, selected, scroll ...) and the meaning of the action codes.
 * The code is the former inline painting of gui/app.c, moved unchanged so the renders stay byte identical. */
#include "w2k.h"
#include <stdlib.h>
#include <string.h>
#include <wchar.h>

/* ---- hit regions ---------------------------------------------------------------------------------------------- */
void w2k_hot_reset(w2k_hotlist *l) { l->n = 0; }
void w2k_hot_add(w2k_hotlist *l, int x, int y, int w, int h, int act, int arg) {
    if (l->n < W2K_HOT_MAX) { w2k_hot *t = &l->v[l->n++]; t->x = x; t->y = y; t->w = w; t->h = h; t->act = act; t->arg = arg; }
}
int w2k_hot_at(const w2k_hotlist *l, int x, int y) {
    for (int i = l->n - 1; i >= 0; i--) { const w2k_hot *t = &l->v[i]; if (x >= t->x && x < t->x + t->w && y >= t->y && y < t->y + t->h) return i; }
    return -1;
}

/* ---- text helpers --------------------------------------------------------------------------------------------- */
int w2k_th(const w2k_gfx *g) { return g ? g->text_h : 16; }
int w2k_tw(const w2k_gfx *g, const wchar_t *s, int bold) { return g->text_w(g->ctx, s, (int)wcslen(s), bold); }
void w2k_txt(const w2k_gfx *g, int x, int y, const wchar_t *s, uint32_t rgb, int bold) { g->text(g->ctx, x, y, s, (int)wcslen(s), rgb, bold); }
void w2k_fillr(const w2k_gfx *g, int x, int y, int w, int h, uint32_t rgb) { if (w > 0 && h > 0) g->fill(g->ctx, x, y, w, h, rgb); }

int w2k_wrap(const w2k_gfx *g, const wchar_t *s, int width, int bold, int *st, int *ln, int max) {
    int n = 0, len = (int)wcslen(s), i = 0;
    while (i < len && n < max) {
        int j = i, last_sp = -1, w = 0;
        while (j < len) {
            int cw = g->text_w(g->ctx, s + j, 1, bold);
            if (w + cw > width && j > i) break;
            if (s[j] == L' ') last_sp = j;
            w += cw; j++;
        }
        int end = j;
        if (j < len && last_sp > i) end = last_sp + 1;
        st[n] = i; ln[n] = end - i; n++;
        i = end;
    }
    if (!n) { st[0] = 0; ln[0] = 0; n = 1; }
    return n;
}
int w2k_draw_wrapped(const w2k_gfx *g, int x, int y, int w, const wchar_t *s, uint32_t c, int bold) {
    int st[64], ln[64], n = w2k_wrap(g, s, w, bold, st, ln, 64);
    for (int i = 0; i < n; i++) g->text(g->ctx, x, y + i * (w2k_th(g) + 1), s + st[i], ln[i], c, bold);
    return n * (w2k_th(g) + 1);
}
int w2k_wrapped_h(const w2k_gfx *g, int w, const wchar_t *s) { int st[64], ln[64]; return w2k_wrap(g, s, w, 0, st, ln, 64) * (w2k_th(g) + 1); }

/* ---- buttons and choices -------------------------------------------------------------------------------------- */
void w2k_button(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int w, int h, const wchar_t *label, int act, int arg, unsigned flags) {
    w2k_draw_button(g, x, y, w, h, label, !!(flags & W2K_PRESSED), !!(flags & W2K_DISABLED), !!(flags & W2K_DEFAULT), !!(flags & W2K_FOCUS));
    if (hot && !(flags & W2K_DISABLED)) w2k_hot_add(hot, x, y, w, h, act, arg);
}

void w2k_choice(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int hit_w, int checked, int radio, int disabled, const wchar_t *label, int act, int arg) {
    int th = w2k_th(g);
    w2k_draw_check(g, x, y + (th + 2 - w2k_px(13)) / 2, checked, disabled, radio);
    w2k_txt(g, x + (radio ? w2k_px(13) + w2k_px(6) : w2k_px(19)), y, label, W2K_TEXT, 0);
    if (hot) w2k_hot_add(hot, x, y, hit_w > 0 ? hit_w : w2k_px(13) + w2k_px(10) + w2k_tw(g, label, 0), th + 2, act, arg);
}

/* ---- combo box ------------------------------------------------------------------------------------------------ */
void w2k_combo(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int w, int h, const wchar_t *text, int act, int arg, int disabled) {
    w2k_draw_combo(g, x, y, w, h, text, disabled);
    if (hot && !disabled) w2k_hot_add(hot, x, y, w, h, act, arg);
}

void w2k_droplist(const w2k_gfx *g, w2k_hotlist *hot, int W, int H, int x, int y, int w, w2k_text_fn text, void *ud, int n, int cur, int act_item, int act_close) {
    int rh = w2k_th(g) + w2k_px(6), h = n * rh + 4;
    for (int i = 0; i < n; i++) { int t = w2k_tw(g, text(ud, i), 0) + w2k_px(16); if (t > w) w = t; }
    w2k_fillr(g, x, y, w, h, W2K_FIELD); w2k_draw_edge(g, x, y, w, h, W2K_EDGE_WINDOW);
    for (int i = 0; i < n; i++) {
        int yy = y + 2 + i * rh;
        if (i == cur) { w2k_fillr(g, x + 2, yy, w - 4, rh, W2K_SELECT); w2k_txt(g, x + 6, yy + 3, text(ud, i), 0xFFFFFF, 0); }
        else w2k_txt(g, x + 6, yy + 3, text(ud, i), W2K_TEXT, 0);
        if (hot) w2k_hot_add(hot, x + 2, yy, w - 4, rh, act_item, i);
    }
    if (hot) w2k_hot_add(hot, 0, 0, W, H, act_close, -1);             /* a click anywhere else closes it */
}

/* ---- property sheet --------------------------------------------------------------------------------------------- */
int w2k_tabsheet(const w2k_gfx *g, w2k_hotlist *hot, int tab_x, int y, int page_x, int page_w, int page_bottom, const wchar_t *const *labels, int n, int sel, int act_base) {
    int tabH = w2k_th(g) + w2k_px(10), tx = tab_x, tabw[8], tabx[8];
    if (n > 8) n = 8;
    for (int i = 0; i < n; i++) { tabw[i] = w2k_tw(g, labels[i], 0) + w2k_px(24); tabx[i] = tx; tx += tabw[i] + 2; }
    int pageTop = y + tabH;
    w2k_fillr(g, page_x, pageTop, page_w, page_bottom - pageTop, W2K_FACE);
    w2k_draw_edge(g, page_x, pageTop, page_w, page_bottom - pageTop, W2K_EDGE_WINDOW);
    for (int i = 0; i < n; i++) if (i != sel) { w2k_draw_tab(g, tabx[i], y + 2, tabw[i], tabH - 2, labels[i], 0); if (hot) w2k_hot_add(hot, tabx[i], y + 2, tabw[i], tabH - 2, act_base + i, 0); }
    w2k_draw_tab(g, tabx[sel], y + 2, tabw[sel], tabH - 2, labels[sel], 1);
    w2k_fillr(g, tabx[sel] - 1, pageTop, tabw[sel] + 2, 2, W2K_FACE);          /* the active tab merges with the page */
    if (hot) w2k_hot_add(hot, tabx[sel], y + 2, tabw[sel], tabH - 2, act_base + sel, 0);
    return pageTop;
}

/* ---- menus -------------------------------------------------------------------------------------------------------- */
void w2k_menubar(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int mh, const wchar_t *const *labels, int n, int open, int act) {
    for (int i = 0; i < n; i++) {
        const wchar_t *s = labels[i]; int w = w2k_tw(g, s, 0) + w2k_px(16);
        if (i == open) w2k_fillr(g, x, y + 1, w, mh - 2, W2K_SELECT);
        w2k_txt(g, x + w2k_px(8), y + (mh - w2k_th(g)) / 2, s, i == open ? 0xFFFFFF : W2K_TEXT, 0);
        if (hot) w2k_hot_add(hot, x, y, w, mh, act, i);
        x += w;
    }
}

void w2k_popup_menu(const w2k_gfx *g, w2k_hotlist *hot, int W, int H, int x, int y, const w2k_menu_item *it, int n, int act_item, int act_close) {
    int rh = w2k_th(g) + w2k_px(6), w = w2k_px(150);
    for (int i = 0; i < n; i++) { int t = w2k_tw(g, it[i].text, 0) + w2k_px(40); if (t > w) w = t; }
    int h = 4; for (int i = 0; i < n; i++) h += it[i].sep ? 6 : rh;
    w2k_fillr(g, x, y, w, h, W2K_FACE); w2k_draw_edge(g, x, y, w, h, W2K_EDGE_RAISED);
    int yy = y + 2;
    for (int i = 0; i < n; i++) {
        if (it[i].sep) { w2k_fillr(g, x + 3, yy + 2, w - 6, 1, W2K_SHADOW); w2k_fillr(g, x + 3, yy + 3, w - 6, 1, W2K_HIGHLIGHT); yy += 6; continue; }
        if (it[i].checked) w2k_txt(g, x + 6, yy + 3, L"•", W2K_TEXT, 0);
        w2k_txt(g, x + w2k_px(20), yy + 3, it[i].text, it[i].code ? W2K_TEXT : W2K_TEXT_DIM, 0);
        if (it[i].code && hot) w2k_hot_add(hot, x + 2, yy, w - 4, rh, act_item, it[i].code);
        yy += rh;
    }
    if (hot) w2k_hot_add(hot, 0, 0, W, H, act_close, -1);              /* a click anywhere else closes it */
}

/* ---- report list -------------------------------------------------------------------------------------------------- */
void w2k_list_select(int *sel, int *scroll, int nrows, int i, int page) {
    if (i < 0) i = 0;
    if (i >= nrows) i = nrows - 1;
    *sel = i;
    if (i < *scroll) *scroll = i;
    if (i >= *scroll + page) *scroll = i - page + 1;
}

void w2k_list_paint(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int w, int h, const w2k_list *L) {
    int th = w2k_th(g);
    w2k_fillr(g, x, y, w, h, W2K_FIELD); w2k_draw_edge(g, x, y, w, h, W2K_EDGE_SUNKEN);
    int ix = x + 2, iy = y + 2, iw = w - 4, ih = h - 4, hh = th + w2k_px(6), rh = th + 3;
    int page = (ih - hh) / rh; if (page < 1) page = 1;
    int sbw = L->nrows > page ? w2k_px(16) : 0;
    int cw[16], fixed = 0, nc = L->ncols > 16 ? 16 : L->ncols;
    for (int i = 1; i < nc; i++) { cw[i] = w2k_px(L->col_px[i]); fixed += cw[i]; }
    cw[0] = iw - sbw - fixed; if (cw[0] < w2k_px(L->col_px[0])) cw[0] = w2k_px(L->col_px[0]);
    g->clip(g->ctx, ix, iy, iw - sbw, ih);
    int cx = ix;
    for (int i = 0; i < nc; i++) {
        w2k_draw_header_cell(g, cx, iy, cw[i], hh, L->headers[i]);
        g->clip(g->ctx, ix, iy, iw - sbw, ih);
        if (hot) w2k_hot_add(hot, cx, iy, cw[i], hh, L->act_hdr, i);
        cx += cw[i];
    }
    int *scroll = L->scroll;
    if (*scroll > L->nrows - page) *scroll = L->nrows - page;
    if (*scroll < 0) *scroll = 0;
    for (int r = 0; r < page && *scroll + r < L->nrows; r++) {
        int vi = *scroll + r, ry = iy + hh + r * rh;
        if (vi == L->sel) w2k_fillr(g, ix, ry, iw - sbw, rh, W2K_ROW_SEL);
        else if (r & 1) w2k_fillr(g, ix, ry, iw - sbw, rh, W2K_ROW_ALT);
        cx = ix;
        for (int i = 0; i < nc; i++) { g->clip(g->ctx, cx + 1, ry, cw[i] - 2, rh); w2k_txt(g, cx + 6, ry + 1, L->cell(L->ud, vi, i), W2K_TEXT, 0); cx += cw[i]; }
        g->clip(g->ctx, ix, iy, iw - sbw, ih);
        if (hot) w2k_hot_add(hot, ix, ry, iw - sbw, rh, L->act_row, vi);
    }
    g->clip(g->ctx, 0, 0, 0, 0);
    if (sbw) { w2k_draw_scrollbar(g, ix + iw - sbw, iy + hh, sbw, ih - hh, *scroll, page, L->nrows); if (hot) w2k_hot_add(hot, ix + iw - sbw, iy + hh, sbw, ih - hh, L->act_scroll, 0); }
}

/* ---- text view ------------------------------------------------------------------------------------------------------ */
typedef struct { const wchar_t *s; int n; uint32_t c; int bold, indent; } VLine;

void w2k_textview(const w2k_gfx *g, int x, int y, int w, int h, const w2k_line *L, int count, int *scroll, int stick_bottom) {
    w2k_fillr(g, x, y, w, h, W2K_FIELD); w2k_draw_edge(g, x, y, w, h, W2K_EDGE_SUNKEN);
    int sbw = w2k_px(16), lh = w2k_th(g) + 1, page = (h - 6) / lh; if (page < 1) page = 1;
    int width = w - 8 - sbw, cap = 0, vn = 0; VLine *vl = NULL;
    for (int i = 0; i < count; i++) {
        int ind = L[i].indent * w2k_px(16), st[128], ln[128], n = w2k_wrap(g, L[i].text, width - ind, L[i].bold, st, ln, 128);
        for (int k = 0; k < n; k++) {
            if (vn == cap) { cap = cap ? cap * 2 : 128; vl = (VLine *)realloc(vl, cap * sizeof(VLine)); }
            vl[vn].s = L[i].text + st[k]; vl[vn].n = ln[k]; vl[vn].c = L[i].rgb; vl[vn].bold = L[i].bold; vl[vn].indent = ind; vn++;
        }
    }
    if (stick_bottom && vn > page) *scroll = vn - page;
    if (*scroll > vn - page) *scroll = vn - page;
    if (*scroll < 0) *scroll = 0;
    g->clip(g->ctx, x + 2, y + 2, w - 4 - sbw, h - 4);
    for (int i = 0; i < page && *scroll + i < vn; i++) {
        VLine *v = &vl[*scroll + i];
        g->text(g->ctx, x + 4 + v->indent, y + 3 + i * lh, v->s, v->n, v->c, v->bold);
    }
    g->clip(g->ctx, 0, 0, 0, 0);
    if (vn > page) w2k_draw_scrollbar(g, x + w - sbw - 2, y + 2, sbw, h - 4, *scroll, page, vn);
    free(vl);
}

/* ---- status bar, dialog frame, window caption ------------------------------------------------------------------------- */
void w2k_statusbar(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *const *t, int n) {
    int gap = 2, grip = w2k_px(14), rest = w - grip - (n - 1) * gap, wd[16];
    if (n > 16) n = 16;
    for (int i = 1; i < n; i++) { wd[i] = w2k_tw(g, t[i], 0) + w2k_px(16); rest -= wd[i]; }
    wd[0] = rest < 0 ? 0 : rest;
    int sx = x;
    for (int i = 0; i < n; i++) { w2k_draw_pane(g, sx, y + 2, wd[i], h - 4, t[i], 1); sx += wd[i] + gap; }
    for (int i = 0; i < 3; i++) { int o = i * 4; w2k_fillr(g, x + w - grip + 2 + o, y + h - 4, 10 - o, 1, W2K_SHADOW); w2k_fillr(g, x + w - grip + 3 + o, y + h - 3, 10 - o, 1, W2K_HIGHLIGHT); }
}

int w2k_dialog_frame(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *title) {
    w2k_fillr(g, x, y, w, h, W2K_FACE); w2k_draw_edge(g, x, y, w, h, W2K_EDGE_WINDOW);
    int capH = w2k_th(g) + w2k_px(6);
    w2k_draw_caption(g, x + 3, y + 3, w - 6, capH, title, 1);
    return capH;
}

int w2k_window_caption(const w2k_gfx *g, w2k_hotlist *hot, int W, int H, const wchar_t *title, int act, int pressed_code) {
    w2k_draw_edge(g, 0, 0, W, H, W2K_EDGE_WINDOW);
    int capH = w2k_th(g) + w2k_px(6), bs = capH - 4;
    w2k_draw_caption(g, 3, 3, W - 6, capH, title, 1);
    int x = W - 5 - bs;
    for (int i = 2; i >= 0; i--) { w2k_draw_caption_button(g, x, 5, bs, bs, i, pressed_code == act * 10 + i); if (hot) w2k_hot_add(hot, x, 5, bs, bs, act, i); x -= bs + (i == 2 ? w2k_px(2) : 0); }
    return 3 + capH;
}
