/* libw2k Linux backend: software canvas, PNG I/O, snapshot comparison. */
#define STB_TRUETYPE_IMPLEMENTATION
#include "third_party/stb_truetype.h"
#include "w2k_png.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <zlib.h>

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

int w2k_png_font(const char *path, int index, int px_size) {
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
static uint32_t crc_table[256];
static uint32_t crc(uint32_t c, const uint8_t *p, size_t n) { for (size_t i = 0; i < n; i++) c = crc_table[(c ^ p[i]) & 255] ^ (c >> 8); return c; }
static void be32(uint8_t *p, uint32_t v) { p[0] = v >> 24; p[1] = v >> 16; p[2] = v >> 8; p[3] = v; }
static void chunk(FILE *f, const char *type, const uint8_t *d, uint32_t n) {
    uint8_t h[8]; be32(h, n); memcpy(h + 4, type, 4); fwrite(h, 1, 8, f);
    if (n) fwrite(d, 1, n, f);
    uint32_t c = crc(0xFFFFFFFFu, (const uint8_t *)type, 4); c = crc(c, d, n) ^ 0xFFFFFFFFu; uint8_t t[4]; be32(t, c); fwrite(t, 1, 4, f);
}
int w2k_png_write_rgb(const char *path, const uint8_t *rgb, int W, int H) {
    for (int i = 0; i < 256; i++) { uint32_t c = i; for (int k = 0; k < 8; k++) c = c & 1 ? 0xEDB88320u ^ (c >> 1) : c >> 1; crc_table[i] = c; }
    size_t rl = 1 + 3 * (size_t)W, n = rl * H; uint8_t *raw = (uint8_t *)malloc(n);
    for (int y = 0; y < H; y++) { raw[y * rl] = 0; memcpy(raw + y * rl + 1, rgb + 3 * (size_t)y * W, 3 * (size_t)W); }
    uLongf zl = compressBound((uLong)n); uint8_t *z = (uint8_t *)malloc(zl);
    if (compress2(z, &zl, raw, (uLong)n, 9) != Z_OK) { free(raw); free(z); return 0; }
    FILE *f = fopen(path, "wb"); if (!f) { free(raw); free(z); return 0; }
    fwrite("\x89PNG\r\n\x1a\n", 1, 8, f);
    uint8_t ih[13]; be32(ih, W); be32(ih + 4, H); ih[8] = 8; ih[9] = 2; ih[10] = ih[11] = ih[12] = 0; chunk(f, "IHDR", ih, 13);
    chunk(f, "IDAT", z, (uint32_t)zl); chunk(f, "IEND", NULL, 0); fclose(f); free(raw); free(z); return 1;
}
int w2k_png_save(const char *path) { return w2k_png_write_rgb(path, px, CW, CH); }
int w2k_png_text_h(void) { return text_h; }
void w2k_png_begin(int W, int H) { CW = W; CH = H; px = (uint8_t *)realloc(px, 3 * (size_t)W * H); clip_on = 0; memset(px, 0, 3 * (size_t)W * H); }
w2k_gfx w2k_png_gfx(void) { w2k_gfx g = {NULL, v_fill, v_gradient, v_text, v_text_w, v_clip, text_h}; return g; }
/* returns malloc'ed RGB or NULL; only reads PNGs written above */
uint8_t *w2k_png_read(const char *path, int *W, int *H) {
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


int w2k_png_compare(const char *a, const char *b, int tol, double limit, const char *diff_path, char *msg, int msgn, double *pct_out) {
    int w1, h1, w2, h2; uint8_t *x = w2k_png_read(a, &w1, &h1), *y = w2k_png_read(b, &w2, &h2);
    if (!x || !y) { snprintf(msg, msgn, "%s", !y ? "no baseline (run with --update)" : "cannot read the rendered image"); free(x); free(y); return 1; }
    if (w1 != w2 || h1 != h2) { snprintf(msg, msgn, "size %dx%d, baseline %dx%d", w1, h1, w2, h2); free(x); free(y); return 1; }
    long diff = 0, total = (long)w1 * h1; uint8_t *dimg = (uint8_t *)malloc(3 * (size_t)total);
    for (long k = 0; k < total; k++) {
        int d = 0; for (int c = 0; c < 3; c++) { int e = abs((int)x[3 * k + c] - (int)y[3 * k + c]); if (e > d) d = e; }
        if (d > tol) { diff++; dimg[3 * k] = 255; dimg[3 * k + 1] = 0; dimg[3 * k + 2] = 0; } else { dimg[3 * k] = dimg[3 * k + 1] = dimg[3 * k + 2] = (uint8_t)(y[3 * k] / 2 + 120); }
    }
    double pct = 100.0 * diff / total; if (pct_out) *pct_out = pct;
    int bad = pct > limit;
    if (bad) { if (diff_path) w2k_png_write_rgb(diff_path, dimg, w1, h1); snprintf(msg, msgn, "%.2f%% of the pixels differ (limit %.2f%%)%s%s", pct, limit, diff_path ? ", see " : "", diff_path ? diff_path : ""); }
    else snprintf(msg, msgn, "%.3f%% differ", pct);
    free(dimg); free(x); free(y); return bad;
}

int w2k_snap_run(const char *base, const char *dir, const char *const *names, int n, int update) {
    int bad = 0; char a[800], b[800], m[900], dp[800], cmd[1700];
    snprintf(a, sizeof a, "mkdir -p '%s'", base); (void)!system(a);
    for (int i = 0; i < n; i++) {
        snprintf(a, sizeof a, "%s/%s.png", dir, names[i]); snprintf(b, sizeof b, "%s/%s.png", base, names[i]);
        if (update) { snprintf(cmd, sizeof cmd, "cp '%s' '%s'", a, b); (void)!system(cmd); printf("updated %s\n", names[i]); continue; }
        snprintf(dp, sizeof dp, "%s/%s.diff.png", dir, names[i]);
        if (w2k_png_compare(a, b, 24, 0.30, dp, m, sizeof m, NULL)) { printf("FAIL %s: %s\n", names[i], m); bad++; }
        else printf("ok   %s (%s)\n", names[i], m);
    }
    return bad ? 1 : 0;
}
