/* Platform independent application: state, layout, painting, hit-testing and the actions. A host (win_main.c on
 * Windows, preview_main.c on Linux) provides the window, the native EDIT fields, timers and the calls into the core. */
#ifndef APP_H
#define APP_H
#include "gfx.h"
#include <stdint.h>

enum { EDIT_SEARCH, EDIT_COPYNAME, EDIT_RENAME, EDIT_COUNT };

typedef struct host {
    void (*call_async)(int id, const char *method, const char *args_json); /* answer -> app_on_result(id, json) */
    char *(*call_sync)(const char *method, const char *args_json);          /* malloc'ed JSON; free() it */
    void (*edit_place)(int edit, int x, int y, int w, int h, int visible);
    void (*edit_get)(int edit, wchar_t *buf, int n);
    void (*edit_set)(int edit, const wchar_t *s);
    void (*invalidate)(void);
    void (*window_cmd)(int cmd);                    /* 0 minimise 1 maximise/restore 2 close */
    void (*set_timer)(int ms);
    void (*set_font)(const char *face, int px);     /* host rebuilds its font; then app repaints */
    void (*quit)(int code);
} host_t;

void app_init(host_t *h, int frameless, const char *lang, int font_px);
void app_paint(gfx_t *g, int w, int h);
void app_mouse(int kind, int x, int y);             /* 0 down 1 up 2 move */
void app_wheel(int x, int y, int delta);
void app_key(int vk);                               /* Windows virtual-key codes */
void app_tick(void);
void app_on_result(int id, const char *json);
void app_edit_changed(int edit);
int  app_hit_caption(int x, int y);                 /* 1 = draggable caption area */
void app_set_lang(const char *tag);
void app_set_font(int px);
const char *app_caption_text(void);
/* preview/self-test helpers */
void app_enable_selftest(const char *dir);
void app_preview_state(int what);                   /* see preview_main.c */
#endif
