/* libw2k demo and tests on Linux: draws every control on one page, renders it to PNG, compares with libw2k/tests/snapshots and
 * runs the logic checks of the hit-region list and the list scrolling. No application code, no Go core.
 *   w2k_demo --render DIR | --check BASE DIR | --update BASE DIR | --unit */
#include "w2k.h"
#include "w2k_png.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static const char *FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf";
static w2k_hotlist HOT;

static const wchar_t *cell(void *ud, int r, int c) {
    static wchar_t b[3][32]; static int k; (void)ud; k = (k + 1) % 3;
    swprintf(b[k], 32, c == 0 ? L"Model %d" : L"%d-%02d", 1000 + r, c * 7 + r % 9);
    return b[k];
}
static const wchar_t *dropn(void *ud, int i) { static const wchar_t *t[] = {L"FM1  192.168.1.61", L"FM2  192.168.1.62", L"BM   192.168.1.63"}; (void)ud; return t[i]; }

static void page(const w2k_gfx *g, int W, int H, int popup) {
    int p = w2k_px(1); (void)p;
    w2k_hot_reset(&HOT);
    w2k_fillr(g, 0, 0, W, H, W2K_FACE);
    int y0 = w2k_window_caption(g, &HOT, W, H, L"libw2k gallery", 1, -1);
    static const wchar_t *menus[] = {L"File", L"Edit", L"View", L"Help"};
    w2k_menubar(g, &HOT, 0, y0, w2k_px(20), menus, 4, popup ? 1 : -1, 10);
    int y = y0 + w2k_px(28), x = w2k_px(12);
    static const wchar_t *tabs[] = {L"Copy", L"Rename", L"Export"};
    int top = w2k_tabsheet(g, &HOT, x, y, x, W - 2 * x, H - w2k_px(40), tabs, 3, 0, 20);

    int bx = x + w2k_px(12), by = top + w2k_px(12), bw = w2k_px(84), bh = w2k_px(23), gap = w2k_px(8);
    w2k_button(g, &HOT, bx, by, bw, bh, L"Normal", 1, 0, 0);
    w2k_button(g, &HOT, bx + (bw + gap), by, bw, bh, L"Default", 2, 0, W2K_DEFAULT | W2K_FOCUS);
    w2k_button(g, &HOT, bx + 2 * (bw + gap), by, bw, bh, L"Pressed", 3, 0, W2K_PRESSED);
    w2k_button(g, &HOT, bx + 3 * (bw + gap), by, bw, bh, L"Disabled", 4, 0, W2K_DISABLED);
    int cy = by + bh + w2k_px(14);
    w2k_choice(g, &HOT, bx, cy, 0, 1, 0, 0, L"Checked", 5, 0);
    w2k_choice(g, &HOT, bx + w2k_px(110), cy, 0, 0, 0, 0, L"Unchecked", 5, 1);
    w2k_choice(g, &HOT, bx + w2k_px(230), cy, 0, 1, 1, 0, L"Radio on", 6, 0);
    w2k_choice(g, &HOT, bx + w2k_px(340), cy, 0, 0, 1, 1, L"Radio off, disabled", 6, 1);
    int gy = cy + w2k_px(26);
    w2k_draw_group(g, bx, gy, w2k_px(280), w2k_px(150), L"Machine");
    w2k_combo(g, &HOT, bx + w2k_px(12), gy + w2k_px(22), w2k_px(200), w2k_px(21), L"FM1  192.168.1.61", 7, 0, 0);
    w2k_combo(g, &HOT, bx + w2k_px(12), gy + w2k_px(52), w2k_px(200), w2k_px(21), L"disabled", 7, 1, 1);
    w2k_draw_field(g, bx + w2k_px(12), gy + w2k_px(84), w2k_px(200), w2k_px(21));
    w2k_txt(g, bx + w2k_px(18), gy + w2k_px(86), L"text field", W2K_TEXT, 0);
    w2k_draw_progress(g, bx + w2k_px(12), gy + w2k_px(116), w2k_px(250), w2k_px(18), 62);

    int lx = bx + w2k_px(300), ly = gy;
    static const wchar_t *hdr[] = {L"Model", L"Date", L"State"};
    static const int cols[3] = {120, 72, 72};
    static int scroll, sel = 2;
    w2k_list L = {3, cols, hdr, 40, cell, NULL, sel, &scroll, 30, 31, 32};
    w2k_list_paint(g, &HOT, lx, ly, W - lx - x - w2k_px(12), w2k_px(150), &L);

    static const w2k_line lines[] = {{L"Plan: copy 3 folders", W2K_TEXT, 1, 0}, {L"  + 12 files will be created", W2K_OK, 0, 1},
                                     {L"  ! 2 files will be replaced (older on the target)", W2K_WARN, 0, 1}, {L"  - 1 folder will be deleted", W2K_NG, 0, 1}};
    w2k_textview(g, bx, gy + w2k_px(164), W - bx - x - w2k_px(12), w2k_px(96), lines, 4, &(int){0}, 0);
    static const wchar_t *st[] = {L"Ready", L"FM1 192.168.1.61", L"3 models"};
    w2k_statusbar(g, 0, H - w2k_px(22), W, w2k_px(22), st, 3);

    if (popup) {
        static const w2k_menu_item items[] = {{L"Open", 1, 0, 0}, {L"Settings", 2, 1, 0}, {L"", 0, 0, 1}, {L"Exit", 3, 0, 0}};
        w2k_popup_menu(g, &HOT, W, H, x, y0 + w2k_px(20), items, 4, 40, 41);
        w2k_droplist(g, &HOT, W, H, bx + w2k_px(12), gy + w2k_px(43), w2k_px(200), dropn, NULL, 3, 1, 42, 43);
    }
    int dh = w2k_dialog_frame(g, W - w2k_px(250), H - w2k_px(110), w2k_px(230), w2k_px(80), L"Dialog");
    (void)dh;
}

static int render(const char *dir, const char *name, int px, int popup) {
    char path[700];
    w2k_font_px = px;
    if (!w2k_png_font(FONT, 0, px)) { fprintf(stderr, "font %s not found (fonts-dejavu-core)\n", FONT); return 0; }
    int W = 900 * px / 14, H = 560 * px / 14;
    w2k_png_begin(W, H); w2k_gfx g = w2k_png_gfx(); page(&g, W, H, popup);
    snprintf(path, sizeof path, "%s/%s.png", dir, name);
    return w2k_png_save(path);
}

static int fails;
#define CHECK(c) do { if (!(c)) { printf("FAIL %s:%d %s\n", __FILE__, __LINE__, #c); fails++; } } while (0)
static int unit(void) {
    w2k_hotlist h; w2k_hot_reset(&h);
    w2k_hot_add(&h, 0, 0, 100, 100, 1, 0); w2k_hot_add(&h, 10, 10, 20, 20, 2, 7);
    CHECK(h.n == 2); CHECK(w2k_hot_at(&h, 15, 15) == 1); CHECK(w2k_hot_at(&h, 50, 50) == 0); CHECK(w2k_hot_at(&h, 150, 5) == -1);
    int sel = 0, scroll = 0;
    w2k_list_select(&sel, &scroll, 50, 30, 10); CHECK(sel == 30); CHECK(scroll == 21);
    w2k_list_select(&sel, &scroll, 50, 5, 10); CHECK(scroll == 5);
    w2k_list_select(&sel, &scroll, 50, 99, 10); CHECK(sel == 49);
    w2k_list_select(&sel, &scroll, 50, -4, 10); CHECK(sel == 0 && scroll == 0);
    w2k_font_px = 14;
    CHECK(w2k_px(13) == 14); CHECK(w2k_px(0) == 0);
    w2k_font_px = 26; CHECK(w2k_px(13) == 26);
    printf(fails ? "unit: %d failures\n" : "unit: ok\n", fails);
    return fails != 0;
}

int main(int argc, char **argv) {
    static const char *const names[] = {"gallery-14", "gallery-14-popup", "gallery-18"};
    if (argc > 1 && !strcmp(argv[1], "--unit")) return unit();
    if (argc > 2 && !strcmp(argv[1], "--render")) {
        return !(render(argv[2], names[0], 14, 0) && render(argv[2], names[1], 14, 1) && render(argv[2], names[2], 18, 0));
    }
    if (argc > 3 && (!strcmp(argv[1], "--check") || !strcmp(argv[1], "--update"))) {
        if (!(render(argv[3], names[0], 14, 0) && render(argv[3], names[1], 14, 1) && render(argv[3], names[2], 18, 0))) return 2;
        return w2k_snap_run(argv[2], argv[3], names, 3, !strcmp(argv[1], "--update"));
    }
    fprintf(stderr, "usage: w2k_demo --unit | --render DIR | --check BASE DIR | --update BASE DIR\n");
    return 2;
}
