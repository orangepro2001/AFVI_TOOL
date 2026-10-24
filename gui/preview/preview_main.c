/* Linux preview: renders the REAL application (app.c + skin.c, the same code the Windows exe runs) into PNG files with a
 * software canvas, against the real Go core (native c-archive) on throw-away local machines. This is how the look can be
 * reviewed without Windows. Text uses stb_truetype with system fonts (Tahoma is not available: DejaVu Sans / Noto CJK). */
#define STB_TRUETYPE_IMPLEMENTATION
#include "../app.h"
#include "w2k_png.h"
#include "../i18n.h"
#include "../util.h"
#include "../afvi_core.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

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
static void h_font(const char *face, int pxs_) { (void)face; w2k_png_font(g_font_path, g_font_index, pxs_); }
static void h_quit(int c) { (void)c; }
static host_t HOST = {h_call_async, h_call_sync, h_edit_place, h_edit_get, h_edit_set, h_nop, h_cmd, h_timer, h_font, h_quit};

static void render(const char *path, int W, int H) {
    w2k_png_begin(W, H);
    w2k_gfx g = w2k_png_gfx();
    app_paint(&g, W, H);
    w2k_png_save(path);
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
        w2k_png_font(L[li].font, L[li].idx, font_px);
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
    app_init(&HOST, 0, "en", 14); w2k_png_font(latin_font, 0, 14);
    app_enable_selftest(work);
    for (int i = 0; i < 3000 && quit_code < 0; i++) { app_tick(); usleep(10000); }
    char p[700], line[400] = "no result"; snprintf(p, sizeof p, "%s/selftest-result.txt", work);
    FILE *f = fopen(p, "r"); if (f) { if (!fgets(line, sizeof line, f)) strcpy(line, "empty result"); fclose(f); }
    printf("%s", line); if (!strchr(line, '\n')) printf("\n");
    return quit_code == 0 ? 0 : 1;
}

static int check_snapshots(const char *base, const char *dir, int update) {
    static const char *const names[] = {"main-ko", "main-zh-Hans", "main-zh-Hant", "main-en", "plan-ko", "job-ko", "menu-ko", "combo-ko", "refused-ko",
                                        "plan-en", "job-en", "menu-en", "combo-en", "refused-en"};
    return w2k_snap_run(base, dir, names, (int)(sizeof names / sizeof *names), update);
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
