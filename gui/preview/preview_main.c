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

/* ---- PNG (zlib-compressed, filter 0: only our own files are ever read back) ---------------------------------- */
#include <zlib.h>
static uint32_t crc_table[256];
static uint32_t crc(uint32_t c, const uint8_t *p, size_t n) { for (size_t i = 0; i < n; i++) c = crc_table[(c ^ p[i]) & 255] ^ (c >> 8); return c; }
static void be32(uint8_t *p, uint32_t v) { p[0] = v >> 24; p[1] = v >> 16; p[2] = v >> 8; p[3] = v; }
static void chunk(FILE *f, const char *type, const uint8_t *d, uint32_t n) {
    uint8_t h[8]; be32(h, n); memcpy(h + 4, type, 4); fwrite(h, 1, 8, f);
    if (n) fwrite(d, 1, n, f);
    uint32_t c = crc(0xFFFFFFFFu, (const uint8_t *)type, 4); c = crc(c, d, n) ^ 0xFFFFFFFFu; uint8_t t[4]; be32(t, c); fwrite(t, 1, 4, f);
}
static void write_rgb_png(const char *path, const uint8_t *rgb, int W, int H) {
    for (int i = 0; i < 256; i++) { uint32_t c = i; for (int k = 0; k < 8; k++) c = c & 1 ? 0xEDB88320u ^ (c >> 1) : c >> 1; crc_table[i] = c; }
    size_t rl = 1 + 3 * (size_t)W, n = rl * H; uint8_t *raw = (uint8_t *)malloc(n);
    for (int y = 0; y < H; y++) { raw[y * rl] = 0; memcpy(raw + y * rl + 1, rgb + 3 * (size_t)y * W, 3 * (size_t)W); }
    uLongf zl = compressBound((uLong)n); uint8_t *z = (uint8_t *)malloc(zl);
    if (compress2(z, &zl, raw, (uLong)n, 9) != Z_OK) { fprintf(stderr, "zlib failed\n"); exit(1); }
    FILE *f = fopen(path, "wb"); if (!f) { fprintf(stderr, "cannot write %s\n", path); exit(1); }
    fwrite("\x89PNG\r\n\x1a\n", 1, 8, f);
    uint8_t ih[13]; be32(ih, W); be32(ih + 4, H); ih[8] = 8; ih[9] = 2; ih[10] = ih[11] = ih[12] = 0; chunk(f, "IHDR", ih, 13);
    chunk(f, "IDAT", z, (uint32_t)zl); chunk(f, "IEND", NULL, 0); fclose(f); free(raw); free(z);
}
static void write_png(const char *path) { write_rgb_png(path, px, CW, CH); }
/* returns malloc'ed RGB or NULL; only reads PNGs written above */
static uint8_t *read_rgb_png(const char *path, int *W, int *H) {
    FILE *f = fopen(path, "rb"); if (!f) return NULL;
    fseek(f, 0, SEEK_END); long n = ftell(f); fseek(f, 0, SEEK_SET); uint8_t *d = (uint8_t *)malloc(n);
    if (fread(d, 1, n, f) != (size_t)n) { fclose(f); free(d); return NULL; } fclose(f);
    uint8_t *idat = (uint8_t *)malloc(n); size_t il = 0; long o = 8; int w = 0, h = 0;
    while (o + 12 <= n) {
        uint32_t len = ((uint32_t)d[o] << 24) | (d[o + 1] << 16) | (d[o + 2] << 8) | d[o + 3];
        if (!memcmp(d + o + 4, "IHDR", 4)) { w = (int)(((uint32_t)d[o + 8] << 24) | (d[o + 9] << 16) | (d[o + 10] << 8) | d[o + 11]); h = (int)(((uint32_t)d[o + 12] << 24) | (d[o + 13] << 16) | (d[o + 14] << 8) | d[o + 15]); }
        else if (!memcmp(d + o + 4, "IDAT", 4)) { memcpy(idat + il, d + o + 8, len); il += len; }
        o += 12 + len;
    }
    free(d); if (w <= 0 || h <= 0) { free(idat); return NULL; }
    size_t rl = 1 + 3 * (size_t)w; uLongf rawl = rl * h; uint8_t *raw = (uint8_t *)malloc(rawl);
    if (uncompress(raw, &rawl, idat, (uLong)il) != Z_OK || rawl != rl * h) { free(idat); free(raw); return NULL; }
    free(idat); uint8_t *rgb = (uint8_t *)malloc(3 * (size_t)w * h);
    for (int y = 0; y < h; y++) { if (raw[y * rl] != 0) { free(raw); free(rgb); return NULL; } memcpy(rgb + 3 * (size_t)y * w, raw + y * rl + 1, 3 * (size_t)w); }
    free(raw); *W = w; *H = h; return rgb;
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

static const char *latin_font = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", *cjk_font = "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc";

/* every screen the snapshot tests cover: main window in four languages; plan, job, menu, combo, refusal in ko and en */
static void render_all(const char *dir, int frameless, int font_px) {
    char p[700];
    struct { const char *tag; const char *font; int idx; } L[4] = {{"ko", cjk_font, 1}, {"zh-Hans", cjk_font, 2}, {"zh-Hant", cjk_font, 3}, {"en", latin_font, 0}};
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
}

/* the app's own scripted journey (list, search, languages, copy with the red deletion step, group rename, refusal, Excel entry)
 * against the real core; exit code 0 = passed */
static int quit_code = -1;
static void t_quit(int c) { quit_code = c; }
static int run_selftest(const char *work) {
    HOST.quit = t_quit;
    if (afvi_init(NULL, work, 0) != 0) { fprintf(stderr, "core init failed\n"); return 2; }
    g_font_path = latin_font; g_font_index = 0;
    app_init(&HOST, 0, "en", 14); load_font(latin_font, 0, 14);
    app_enable_selftest(work);
    for (int i = 0; i < 3000 && quit_code < 0; i++) { app_tick(); usleep(10000); }
    char p[700], line[400] = "no result"; snprintf(p, sizeof p, "%s/selftest-result.txt", work);
    FILE *f = fopen(p, "r"); if (f) { if (!fgets(line, sizeof line, f)) strcpy(line, "empty result"); fclose(f); }
    printf("%s", line); if (!strchr(line, '\n')) printf("\n");
    return quit_code == 0 ? 0 : 1;
}

/* compare the freshly rendered PNGs with the committed baselines; tolerant of tiny anti-aliasing differences between font builds */
static int check_snapshots(const char *base, const char *dir, int update) {
    static const char *names[] = {"main-ko", "main-zh-Hans", "main-zh-Hant", "main-en", "plan-ko", "job-ko", "menu-ko", "combo-ko", "refused-ko",
                                  "plan-en", "job-en", "menu-en", "combo-en", "refused-en"};
    int bad = 0; char a[800], b[800];
    snprintf(a, sizeof a, "mkdir -p '%s'", base); (void)!system(a);
    for (size_t i = 0; i < sizeof names / sizeof *names; i++) {
        snprintf(a, sizeof a, "%s/%s.png", dir, names[i]); snprintf(b, sizeof b, "%s/%s.png", base, names[i]);
        if (update) { char cmd[1700]; snprintf(cmd, sizeof cmd, "cp '%s' '%s'", a, b); (void)!system(cmd); printf("updated %s\n", names[i]); continue; }
        int w1, h1, w2, h2; uint8_t *x = read_rgb_png(a, &w1, &h1), *y = read_rgb_png(b, &w2, &h2);
        if (!x || !y) { printf("FAIL %s: %s\n", names[i], !y ? "no baseline (run scripts/test-gui.sh --update)" : "cannot read the rendered image"); bad++; free(x); free(y); continue; }
        if (w1 != w2 || h1 != h2) { printf("FAIL %s: size %dx%d, baseline %dx%d\n", names[i], w1, h1, w2, h2); bad++; free(x); free(y); continue; }
        long diff = 0, total = (long)w1 * h1; uint8_t *dimg = (uint8_t *)malloc(3 * (size_t)total);
        for (long k = 0; k < total; k++) {
            int d = 0; for (int c = 0; c < 3; c++) { int e = abs((int)x[3 * k + c] - (int)y[3 * k + c]); if (e > d) d = e; }
            if (d > 24) { diff++; dimg[3 * k] = 255; dimg[3 * k + 1] = 0; dimg[3 * k + 2] = 0; } else { dimg[3 * k] = dimg[3 * k + 1] = dimg[3 * k + 2] = (uint8_t)(y[3 * k] / 2 + 120); }
        }
        double pct = 100.0 * diff / total;
        if (pct > 0.30) { char dp[800]; snprintf(dp, sizeof dp, "%s/%s.diff.png", dir, names[i]); write_rgb_png(dp, dimg, w1, h1); printf("FAIL %s: %.2f%% of the pixels differ (limit 0.30%%), see %s\n", names[i], pct, dp); bad++; }
        else printf("ok   %s (%.3f%% differ)\n", names[i], pct);
        free(dimg); free(x); free(y);
    }
    return bad ? 1 : 0;
}

int main(int argc, char **argv) {
    if (argc > 2 && !strcmp(argv[1], "--selftest")) return run_selftest(argv[2]);
    if (argc > 3 && (!strcmp(argv[1], "--check") || !strcmp(argv[1], "--update"))) {
        /* --check|--update <baseline dir> <work dir> */
        const char *base = argv[2], *work = argv[3]; char dir[700];
        snprintf(dir, sizeof dir, "%s/render", work); { char c[800]; snprintf(c, sizeof c, "mkdir -p '%s'", dir); (void)!system(c); }
        if (afvi_init(NULL, work, 0) != 0) { fprintf(stderr, "core init failed\n"); return 2; }
        render_all(dir, 1, 14);
        return check_snapshots(base, dir, !strcmp(argv[1], "--update"));
    }
    const char *dir = argc > 1 ? argv[1] : "shots", *work = argc > 2 ? argv[2] : "/tmp/afvi-preview";
    int frameless = argc > 3 ? atoi(argv[3]) : 1, font_px = argc > 4 ? atoi(argv[4]) : 14;
    char p[600]; snprintf(p, sizeof p, "mkdir -p %s", dir); (void)!system(p);
    if (afvi_init(NULL, work, 0) != 0) { fprintf(stderr, "core init failed\n"); return 1; }
    render_all(dir, frameless, font_px);
    return 0;
}
