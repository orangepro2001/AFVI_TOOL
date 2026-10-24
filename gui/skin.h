#ifndef SKIN_H
#define SKIN_H
#include "gfx.h"
/* Windows 2000 tokens, light theme (win2000-style skill, design-tokens.md) */
#define C_FACE      0xD4D0C8u
#define C_HIGHLIGHT 0xFFFFFFu
#define C_LIGHT     0xE8E5DEu
#define C_SHADOW    0x808080u
#define C_DKSHADOW  0x404040u
#define C_FIELD     0xFFFFFFu
#define C_SELECT    0x0A246Au
#define C_CAPTION_L 0x0A246Au
#define C_CAPTION_R 0xA6CAF0u
#define C_TEXT      0x000000u
#define C_TEXT_DIS  0x808080u
#define C_TEXT_DIM  0x404040u
#define C_WARN      0x806000u
#define C_NG        0xC00000u
#define C_OK        0x008000u
#define C_ROW_SEL   0xA6B8D6u
#define C_ROW_ALT   0xEFEFEFu
#define C_DIM_BACK  0x000000u

typedef enum { SKE_RAISED, SKE_SUNKEN, SKE_PRESSED, SKE_STATUS, SKE_ETCHED, SKE_WINDOW } edge_t;

extern int skin_font_px;                 /* design size 13; layout lengths scale with it */
static inline int pxs(int v) { return v * skin_font_px / 13; }

void skin_edge(gfx_t *g, int x, int y, int w, int h, edge_t kind);
void skin_caption(gfx_t *g, int x, int y, int w, int h, const wchar_t *title, int active);
void skin_caption_button(gfx_t *g, int x, int y, int w, int h, int glyph, int pressed); /* 0 min 1 max 2 close */
void skin_button(gfx_t *g, int x, int y, int w, int h, const wchar_t *label, int pressed, int disabled, int is_default, int focus);
void skin_group(gfx_t *g, int x, int y, int w, int h, const wchar_t *title);
void skin_check(gfx_t *g, int x, int y, int checked, int disabled, int radio);
void skin_tab(gfx_t *g, int x, int y, int w, int h, const wchar_t *label, int selected);
void skin_pane(gfx_t *g, int x, int y, int w, int h, const wchar_t *text, int clip);
void skin_field(gfx_t *g, int x, int y, int w, int h);
void skin_combo(gfx_t *g, int x, int y, int w, int h, const wchar_t *text, int disabled);
void skin_progress(gfx_t *g, int x, int y, int w, int h, int percent);
void skin_header_cell(gfx_t *g, int x, int y, int w, int h, const wchar_t *text);
void skin_dotted(gfx_t *g, int x, int y, int w, int h);
void skin_scrollbar(gfx_t *g, int x, int y, int w, int h, int pos, int page, int total);
void skin_text_disabled(gfx_t *g, int x, int y, const wchar_t *s, int n);
#endif
