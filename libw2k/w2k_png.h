/* libw2k backend for Linux tests and previews: a software canvas (stb_truetype text, system fonts) behind w2k_gfx, PNG writing and
 * reading (only the PNGs written here: RGB, filter 0, zlib), and the tolerant image comparison the snapshot tests use.
 * Needs zlib. Not part of the Windows build. */
#ifndef W2K_PNG_H
#define W2K_PNG_H
#include "w2k_gfx.h"
#include <stdint.h>

/* One canvas per process (the tests are single threaded). */
int      w2k_png_font(const char *ttf_or_ttc, int collection_index, int px);   /* 1 = loaded; sets the text height */
int      w2k_png_text_h(void);
void     w2k_png_begin(int width, int height);                                  /* new blank frame */
w2k_gfx  w2k_png_gfx(void);
int      w2k_png_save(const char *path);                                       /* 1 = ok */

uint8_t *w2k_png_read(const char *path, int *width, int *height);                            /* malloc'ed RGB, or NULL */
int      w2k_png_write_rgb(const char *path, const uint8_t *rgb, int width, int height);

/* Compare two PNG files: a pixel differs when any channel differs by more than `channel_tol`. Returns 0 = within limit_pct percent,
 * 1 = too different or unreadable (message in msg), writes a red-on-gray difference image to diff_path when it fails (may be NULL). */
int w2k_png_compare(const char *rendered, const char *baseline, int channel_tol, double limit_pct, const char *diff_path, char *msg, int msgn, double *pct_out);

/* Compare/update a named set of screens: <dir>/<name>.png against <base>/<name>.png. update != 0 copies instead. Prints one line per
 * screen and returns the number of failures. */
int w2k_snap_run(const char *base, const char *dir, const char *const *names, int n, int update);
#endif
