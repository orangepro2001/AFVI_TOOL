#ifndef W2K_H
#define W2K_H
/* libw2k: the Windows 2000 look as a small C99 library. Layer 1 = tokens and primitives (w2k_draw_*), layer 2 = controls
 * (w2k_* below): they paint through w2k_gfx only and register hit regions in a w2k_hotlist; nothing here includes <windows.h>.
 * Backends (layer 0/3): w2k_gdi.h (Windows GDI), w2k_png.h (Linux software canvas + PNG). See libw2k/README.md. */
#include "w2k_gfx.h"
/* Windows 2000 tokens, light theme (win2000-style skill, design-tokens.md) */
#define W2K_FACE      0xD4D0C8u
#define W2K_HIGHLIGHT 0xFFFFFFu
#define W2K_LIGHT     0xE8E5DEu
#define W2K_SHADOW    0x808080u
#define W2K_DKSHADOW  0x404040u
#define W2K_FIELD     0xFFFFFFu
#define W2K_SELECT    0x0A246Au
#define W2K_CAPTION_L 0x0A246Au
#define W2K_CAPTION_R 0xA6CAF0u
#define W2K_TEXT      0x000000u
#define W2K_TEXT_DIS  0x808080u
#define W2K_TEXT_DIM  0x404040u
#define W2K_WARN      0x806000u
#define W2K_NG        0xC00000u
#define W2K_OK        0x008000u
#define W2K_ROW_SEL   0xA6B8D6u
#define W2K_ROW_ALT   0xEFEFEFu
#define W2K_DIM_BACK  0x000000u

typedef enum { W2K_EDGE_RAISED, W2K_EDGE_SUNKEN, W2K_EDGE_PRESSED, W2K_EDGE_STATUS, W2K_EDGE_ETCHED, W2K_EDGE_WINDOW } w2k_edge_t;

extern int w2k_font_px;                 /* design size 13; layout lengths scale with it */
static inline int w2k_px(int v) { return v * w2k_font_px / 13; }

void w2k_draw_edge(const w2k_gfx *g, int x, int y, int w, int h, w2k_edge_t kind);
void w2k_draw_caption(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *title, int active);
void w2k_draw_caption_button(const w2k_gfx *g, int x, int y, int w, int h, int glyph, int pressed); /* 0 min 1 max 2 close */
void w2k_draw_button(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *label, int pressed, int disabled, int is_default, int focus);
void w2k_draw_group(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *title);
void w2k_draw_check(const w2k_gfx *g, int x, int y, int checked, int disabled, int radio);
void w2k_draw_tab(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *label, int selected);
void w2k_draw_pane(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *text, int clip);
void w2k_draw_field(const w2k_gfx *g, int x, int y, int w, int h);
void w2k_draw_combo(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *text, int disabled);
void w2k_draw_progress(const w2k_gfx *g, int x, int y, int w, int h, int percent);
void w2k_draw_header_cell(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *text);
void w2k_draw_dotted(const w2k_gfx *g, int x, int y, int w, int h);
void w2k_draw_scrollbar(const w2k_gfx *g, int x, int y, int w, int h, int pos, int page, int total);
void w2k_draw_text_disabled(const w2k_gfx *g, int x, int y, const wchar_t *s, int n);

/* ---- layer 2: controls ------------------------------------------------------------------------------------------ */

/* Hit regions. Controls append one per clickable area while they paint (act/arg are the application's codes); the host asks
 * w2k_hot_at() for the topmost region under the mouse. Later regions win, so modal layers register last. */
typedef struct { int x, y, w, h, act, arg; } w2k_hot;
#define W2K_HOT_MAX 256
typedef struct { w2k_hot v[W2K_HOT_MAX]; int n; } w2k_hotlist;
void w2k_hot_reset(w2k_hotlist *l);
void w2k_hot_add(w2k_hotlist *l, int x, int y, int w, int h, int act, int arg);
int  w2k_hot_at(const w2k_hotlist *l, int x, int y);                 /* index or -1 */

/* text helpers on a graphics context */
int  w2k_th(const w2k_gfx *g);                                         /* line height */
int  w2k_tw(const w2k_gfx *g, const wchar_t *s, int bold);
void w2k_txt(const w2k_gfx *g, int x, int y, const wchar_t *s, uint32_t rgb, int bold);
void w2k_fillr(const w2k_gfx *g, int x, int y, int w, int h, uint32_t rgb);   /* ignores empty rectangles */
int  w2k_wrap(const w2k_gfx *g, const wchar_t *s, int width, int bold, int *start, int *len, int max);   /* word wrap into pieces */
int  w2k_draw_wrapped(const w2k_gfx *g, int x, int y, int w, const wchar_t *s, uint32_t rgb, int bold);   /* returns the height used */
int  w2k_wrapped_h(const w2k_gfx *g, int w, const wchar_t *s);

/* push button; hit region only when not disabled */
enum { W2K_PRESSED = 1, W2K_DISABLED = 2, W2K_DEFAULT = 4, W2K_FOCUS = 8 };
void w2k_button(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int w, int h, const wchar_t *label, int act, int arg, unsigned flags);

/* check box or radio button with its label; the hit region is always registered (the application decides what a click does).
 * hit_w <= 0: as wide as the label */
void w2k_choice(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int hit_w, int checked, int radio, int disabled, const wchar_t *label, int act, int arg);

/* closed combo box face; the drop-down list is w2k_droplist */
void w2k_combo(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int w, int h, const wchar_t *text, int act, int arg, int disabled);
typedef const wchar_t *(*w2k_text_fn)(void *ud, int index);
void w2k_droplist(const w2k_gfx *g, w2k_hotlist *hot, int W, int H, int x, int y, int w, w2k_text_fn text, void *ud, int n, int cur, int act_item, int act_close);

/* property sheet: tab strip at (tab_x, y), page frame below it; the selected tab merges with the page. Returns the page top.
 * act_base + i is the action of tab i. */
int  w2k_tabsheet(const w2k_gfx *g, w2k_hotlist *hot, int tab_x, int y, int page_x, int page_w, int page_bottom, const wchar_t *const *labels, int n, int selected, int act_base);

/* menu bar and popup menu */
void w2k_menubar(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int h, const wchar_t *const *labels, int n, int open, int act);
typedef struct { const wchar_t *text; int code; int checked, sep; } w2k_menu_item;   /* code 0 = heading (dim, not clickable) */
void w2k_popup_menu(const w2k_gfx *g, w2k_hotlist *hot, int W, int H, int x, int y, const w2k_menu_item *items, int n, int act_item, int act_close);

/* report list: header cells, rows (selected row, alternate rows), vertical scrollbar. Column 0 is flexible, col_px are design widths
 * (scaled with the font; col_px[0] is the minimum of column 0). Regions: headers act_hdr/arg=column, rows act_row/arg=row, scrollbar act_scroll/0. */
typedef struct {
    int ncols; const int *col_px; const wchar_t *const *headers;
    int nrows; const wchar_t *(*cell)(void *ud, int row, int col); void *ud;
    int sel; int *scroll;
    int act_hdr, act_row, act_scroll;
} w2k_list;
void w2k_list_paint(const w2k_gfx *g, w2k_hotlist *hot, int x, int y, int w, int h, const w2k_list *l);
void w2k_list_select(int *sel, int *scroll, int nrows, int i, int page);   /* clamp, then scroll the row into view */

/* log / plan text view: sunken, word wrapped, coloured lines, scrollbar; stick_bottom follows the end */
typedef struct { const wchar_t *text; uint32_t rgb; int bold, indent; } w2k_line;   /* indent in design pixels / 16 steps */
void w2k_textview(const w2k_gfx *g, int x, int y, int w, int h, const w2k_line *lines, int n, int *scroll, int stick_bottom);

/* status bar: sunken panes, the first takes the remaining width, a size grip at the right. y is the top of the bar, h its height */
void w2k_statusbar(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *const *texts, int n);

/* dialog window: Window edge, Face fill, caption bar; returns the caption height (client area starts at y + 3 + that) */
int  w2k_dialog_frame(const w2k_gfx *g, int x, int y, int w, int h, const wchar_t *title);

/* caption bar of a frameless main window with minimise / maximise / close; returns the y where the client area starts.
 * button i has act and arg i; pressed_code == act * 10 + i draws it pressed */
int  w2k_window_caption(const w2k_gfx *g, w2k_hotlist *hot, int W, int H, const wchar_t *title, int act, int pressed_code);
#endif
