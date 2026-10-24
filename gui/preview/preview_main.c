/* Linux preview: renders the REAL application (app.c + skin.c, the same code the Windows exe runs) into PNG files with a
 * software canvas, against the real Go core (native c-archive) on throw-away local machines. This is how the look can be
 * reviewed without Windows. Text uses stb_truetype with system fonts (Tahoma is not available: DejaVu Sans / Noto CJK). */
#define STB_TRUETYPE_IMPLEMENTATION
#include "../third_party/stb_truetype.h"
#include "../app.h"
#include "../skin.h"
#include "../i18n.h"
#include "../util.h"
#include "../afvi_core.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

/* ---- canvas ------------------------------------------------------------------------------------------------- */
static uint8_t *px; static int CW, CH, clipx, clipy, clipw, cliph, clip_on;
static stbtt_fontinfo font; static unsigned char *fontdata; static float fscale;

static void put(int x, int y, uint32_t rgb, int a) {
    if (x < 0 || y < 0 || x >= CW || y >= CH) return;
    if (clip_on && (x < clipx || y < clipy || x >= clipx + clipw || y >= clipy + cliph)) return;
    uint8_t *p = px + 3 * (y * CW + x);
    int r = (rgb >> 16) & 255, g = (rgb >> 8) & 255, b = rgb & 255;
    p[0] = (uint8_t)((p[0] * (255 - a) + r * a) / 255); p[1] = (uint8_t)((p[1] * (255 - a) + g * a) / 255); p[2] = (uint8_t)((p[2] * (255 - a) + b * a) / 255);
}
static void v_fill(void *c, int x, int y, int w, int h, uint32_t rgb) { (void)c; for (int j = 0; j < h; j++) for (int i = 0; i < w; i++) put(x + i, y + j, rgb, 255); }
static void v_gradient(void *c, int x, int y, int w, int h, uint32_t l, uint32_t r) {
    for (int i = 0; i < w; i++) {
        int rr = (int)((l >> 16) & 255) + ((int)((r >> 16) & 255) - (int)((l >> 16) & 255)) * i / (w > 1 ? w - 1 : 1);
        int gg = (int)((l >> 8) & 255) + ((int)((r >> 8) & 255) - (int)((l >> 8) & 255)) * i / (w > 1 ? w - 1 : 1);
        int bb = (int)(l & 255) + ((int)(r & 255) - (int)(l & 255)) * i / (w > 1 ? w - 1 : 1);
        v_fill(c, x + i, y, 1, h, ((uint32_t)rr << 16) | ((uint32_t)gg << 8) | (uint32_t)bb);
    }
}
static int ascent, text_h;
static int glyph_adv(wchar_t ch) { int adv, lsb; stbtt_GetCodepointHMetrics(&font, (int)ch, &adv, &lsb); return (int)(adv * fscale + 0.5f); }
static int v_text_w(void *c, const wchar_t *s, int n, int bold) { (void)c; int w = 0; for (int i = 0; i < n; i++) w += glyph_adv(s[i]); return w + (bold ? 1 : 0); }
static void draw_glyphs(int x, int y, const wchar_t *s, int n, uint32_t rgb) {
    for (int i = 0; i < n; i++) {
        int w, h, xo, yo; unsigned char *bm = stbtt_GetCodepointBitmap(&font, fscale, fscale, (int)s[i], &w, &h, &xo, &yo);
        if (bm) { for (int j = 0; j < h; j++) for (int k = 0; k < w; k++) if (bm[j * w + k]) put(x + xo + k, y + ascent + yo + j, rgb, bm[j * w + k]); stbtt_FreeBitmap(bm, NULL); }
        x += glyph_adv(s[i]);
    }
}
static void v_text(void *c, int x, int y, const wchar_t *s, int n, uint32_t rgb, int bold) { (void)c; draw_glyphs(x, y, s, n, rgb); if (bold) draw_glyphs(x + 1, y, s, n, rgb); }
static void v_clip(void *c, int x, int y, int w, int h) { (void)c; if (w <= 0) clip_on = 0; else { clip_on = 1; clipx = x; clipy = y; clipw = w; cliph = h; } }

static int load_font(const char *path, int index, int px_size) {
    FILE *f = fopen(path, "rb"); if (!f) return 0;
    fseek(f, 0, SEEK_END); long n = ftell(f); fseek(f, 0, SEEK_SET);
    free(fontdata); fontdata = (unsigned char *)malloc(n); if (fread(fontdata, 1, n, f) != (size_t)n) { fclose(f); return 0; } fclose(f);
    int off = stbtt_GetFontOffsetForIndex(fontdata, index);
    if (!stbtt_InitFont(&font, fontdata, off)) return 0;
    fscale = stbtt_ScaleForPixelHeight(&font, (float)px_size);
    int a, d, lg; stbtt_GetFontVMetrics(&font, &a, &d, &lg);
    ascent = (int)(a * fscale + 0.5f); text_h = (int)((a - d) * fscale + 0.5f) + 1;
    return 1;
}

/* ---- PNG (stored deflate blocks) --------------------------------------------------------------------------- */
static uint32_t crc_table[256];
static uint32_t crc(uint32_t c, const uint8_t *p, size_t n) { for (size_t i = 0; i < n; i++) c = crc_table[(c ^ p[i]) & 255] ^ (c >> 8); return c; }
static void be32(uint8_t *p, uint32_t v) { p[0] = v >> 24; p[1] = v >> 16; p[2] = v >> 8; p[3] = v; }
static void chunk(FILE *f, const char *type, const uint8_t *d, uint32_t n) {
    uint8_t h[8]; be32(h, n); memcpy(h + 4, type, 4); fwrite(h, 1, 8, f);
    if (n) fwrite(d, 1, n, f);
    uint32_t c = crc(0xFFFFFFFFu, (const uint8_t *)type, 4); c = crc(c, d, n) ^ 0xFFFFFFFFu; uint8_t t[4]; be32(t, c); fwrite(t, 1, 4, f);
}
static void write_png(const char *path) {
    for (int i = 0; i < 256; i++) { uint32_t c = i; for (int k = 0; k < 8; k++) c = c & 1 ? 0xEDB88320u ^ (c >> 1) : c >> 1; crc_table[i] = c; }
    size_t rl = 1 + 3 * CW, n = rl * CH; uint8_t *raw = (uint8_t *)malloc(n);
    for (int y = 0; y < CH; y++) { raw[y * rl] = 0; memcpy(raw + y * rl + 1, px + 3 * y * CW, 3 * CW); }
    size_t blocks = (n + 65534) / 65535, zl = 2 + n + 5 * blocks + 4; uint8_t *z = (uint8_t *)malloc(zl); size_t o = 0;
    z[o++] = 0x78; z[o++] = 0x01;
    for (size_t b = 0, off = 0; b < blocks; b++) { size_t len = n - off > 65535 ? 65535 : n - off; z[o++] = (b == blocks - 1); z[o++] = len & 255; z[o++] = len >> 8; z[o++] = ~len & 255; z[o++] = (~len >> 8) & 255; memcpy(z + o, raw + off, len); o += len; off += len; }
    uint32_t a = 1, bsum = 0; for (size_t i = 0; i < n; i++) { a = (a + raw[i]) % 65521; bsum = (bsum + a) % 65521; }
    be32(z + o, (bsum << 16) | a); o += 4;
    FILE *f = fopen(path, "wb"); fwrite("\x89PNG\r\n\x1a\n", 1, 8, f);
    uint8_t ih[13]; be32(ih, CW); be32(ih + 4, CH); ih[8] = 8; ih[9] = 2; ih[10] = ih[11] = ih[12] = 0; chunk(f, "IHDR", ih, 13);
    chunk(f, "IDAT", z, (uint32_t)o); chunk(f, "IEND", NULL, 0); fclose(f); free(raw); free(z);
}

/* ---- host: the real core, answered synchronously ------------------------------------------------------------ */
static wchar_t edit_text[EDIT_COUNT][256];
static void h_call_async(int id, const char *m, const char *a) { char *r = afvi_call(m, i18n_tag(), a); app_on_result(id, r); afvi_free(r); }
static char *h_call_sync(const char *m, const char *a) { char *r = afvi_call(m, i18n_tag(), a); char *c = xstrdup(r); afvi_free(r); return c; }
static void h_edit_place(int e, int x, int y, int w, int h, int v) { (void)e; (void)x; (void)y; (void)w; (void)h; (void)v; }
static void h_edit_get(int e, wchar_t *b, int n) { wcsncpy(b, edit_text[e], n - 1); b[n - 1] = 0; }
static void h_edit_set(int e, const wchar_t *s) { wcsncpy(edit_text[e], s, 255); }
static void h_nop(void) {}
static void h_cmd(int c) { (void)c; }
static void h_timer(int ms) { (void)ms; }
static const char *g_font_path; static int g_font_index;
static void h_font(const char *face, int pxs_) { (void)face; load_font(g_font_path, g_font_index, pxs_); }
static void h_quit(int c) { (void)c; }
static host_t HOST = {h_call_async, h_call_sync, h_edit_place, h_edit_get, h_edit_set, h_nop, h_cmd, h_timer, h_font, h_quit};

static void render(const char *path, int W, int H) {
    CW = W; CH = H; px = (uint8_t *)realloc(px, 3 * W * H); clip_on = 0;
    gfx_t g = {NULL, v_fill, v_gradient, v_text, v_text_w, v_clip, text_h};
    app_paint(&g, W, H);
    g.text_h = text_h;
    write_png(path);
    printf("wrote %s\n", path);
}

int main(int argc, char **argv) {
    const char *dir = argc > 1 ? argv[1] : "shots", *work = argc > 2 ? argv[2] : "/tmp/afvi-preview";
    int frameless = argc > 3 ? atoi(argv[3]) : 1, font_px = argc > 4 ? atoi(argv[4]) : 14;
    char p[600]; snprintf(p, sizeof p, "mkdir -p %s", dir); (void)!system(p);
    if (afvi_init(NULL, work, 0) != 0) { fprintf(stderr, "core init failed\n"); return 1; }
    const char *latin = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", *cjk = "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc";
    struct { const char *tag; const char *font; int idx; } L[4] = {{"ko", cjk, 1}, {"zh-Hans", cjk, 2}, {"zh-Hant", cjk, 3}, {"en", latin, 0}};
    int W = 1100 * font_px / 14, H = 720 * font_px / 14;
    for (int li = 0; li < 4; li++) {
        g_font_path = L[li].font; g_font_index = L[li].idx;
        if (li == 0) { app_init(&HOST, frameless, L[0].tag, font_px); } else app_set_lang(L[li].tag);
        load_font(L[li].font, L[li].idx, font_px);
        app_preview_state(8);
        snprintf(p, sizeof p, "%s/main-%s.png", dir, L[li].tag); render(p, W, H);
        if (li == 0 || li == 3) {
            app_preview_state(1); snprintf(p, sizeof p, "%s/plan-%s.png", dir, L[li].tag); render(p, W, H);
            app_preview_state(2); for (int i = 0; i < 60; i++) { app_tick(); usleep(20000); }
            snprintf(p, sizeof p, "%s/job-%s.png", dir, L[li].tag); render(p, W, H);
            app_preview_state(6);
            app_preview_state(3); snprintf(p, sizeof p, "%s/menu-%s.png", dir, L[li].tag); render(p, W, H);
            app_preview_state(6); app_preview_state(4); snprintf(p, sizeof p, "%s/combo-%s.png", dir, L[li].tag); render(p, W, H);
            app_preview_state(6); app_preview_state(5); snprintf(p, sizeof p, "%s/refused-%s.png", dir, L[li].tag); render(p, W, H);
            app_preview_state(6);
        }
    }
    return 0;
}
