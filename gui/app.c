/* Application logic of the Win32 front end (platform independent, see app.h). Everything the user sees is painted
 * here with the Windows 2000 skin; native windows exist only for the three EDIT fields (IME input). */
#include "app.h"
#include "w2k.h"
#include "i18n.h"
#include "util.h"
#include "afvi_core.h"
#include "third_party/cJSON.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <wctype.h>

#define VK_RETURN 13
#define VK_ESCAPE 27
#define VK_PRIOR 33
#define VK_NEXT 34
#define VK_END 35
#define VK_HOME 36
#define VK_UP 38
#define VK_DOWN 40
#define VK_F5 116

/* ---- data ------------------------------------------------------------------------------------ */
typedef struct { char *name, *cls, *mod; int repo, his, insp, light; } Row;
typedef struct { char *text; uint32_t color; int bold, indent; } Line;
typedef struct { Line *v; int n, cap; } Lines;
typedef struct { char *machine, *node; } Ref;

enum { M_NONE, M_PLAN, M_JOB, M_MSG };
enum { P_NONE, P_MENU, P_COMBO };
enum { RQ_LIST = 1, RQ_TARGETS, RQ_PLAN, RQ_START, RQ_JOB, RQ_EXPORT, RQ_CONFIG, RQ_HEALTH };
enum { A_NONE, A_TAB0, A_TAB1, A_REFRESH, A_COPY, A_RENAME, A_MODE_SINGLE, A_MODE_GROUP, A_CHK_RENAME,
       A_COMBO, A_MENU, A_POPITEM, A_CAP, A_EXPORT, A_HDR, A_ROW, A_DLG_RUN, A_DLG_CANCEL, A_DLG_CLOSE, A_DLG_OK, A_SCROLL };
enum { CB_MACHINE, CB_NODE, CB_TSINGLE, CB_TGROUP };


static struct {
    host_t *h;
    int frameless, page, W, H;
    /* data */
    char **machines; int nmach, mach, node;
    Row *rows; int nrows; int *vis; int nvis; int sel_vis; int scroll;
    Ref *single; int nsingle, tsingle; char **group; int ngroup, tgroup;
    int read_only, lock_open; char path[512];
    int mode_group, rename_group, sort_col, sort_desc;
    int list_seq;
    /* status */
    char st_key[32], st_arg[3][256]; int st_n;
    /* ui */
    w2k_hotlist hot; int pressed_hot;
    int modal, popup, popup_kind, popup_arg, popup_x, popup_y, popup_w;
    char version[32];
    /* dialogs */
    char dlg_title[256]; Lines dlg_lines; char dlg_summary[128]; int dlg_can_run, dlg_scroll;
    char plan_id[64]; char job_id[64]; int job_running, job_committed, job_failed, job_done;
    char job_phase[256], job_current[256], job_counts[256], job_result[512]; Lines job_log; int job_pct;
    int busy, draw_edit_text;
    int pending_kind;           /* what the plan was for: 0 copy, 1 rename */
    /* self-test */
    int st_active, st_step, st_wait; char st_dir[512]; char st_msg[256];
} S;

#define EDIT_TEXTLEN 256
static wchar_t edit_cache[EDIT_COUNT][EDIT_TEXTLEN];

/* ---- small helpers ------------------------------------------------------------------------------ */
static void lines_free(Lines *l) { for (int i = 0; i < l->n; i++) free(l->v[i].text); free(l->v); memset(l, 0, sizeof *l); }
static void lines_add(Lines *l, const char *t, uint32_t c, int bold, int indent) {
    if (l->n == l->cap) { l->cap = l->cap ? l->cap * 2 : 32; l->v = (Line *)realloc(l->v, l->cap * sizeof(Line)); }
    l->v[l->n].text = xstrdup(t); l->v[l->n].color = c; l->v[l->n].bold = bold; l->v[l->n].indent = indent; l->n++;
}
static const char *jstr(cJSON *o, const char *k) { cJSON *v = cJSON_GetObjectItemCaseSensitive(o, k); return cJSON_IsString(v) ? v->valuestring : ""; }
static int jint(cJSON *o, const char *k) { cJSON *v = cJSON_GetObjectItemCaseSensitive(o, k); return cJSON_IsNumber(v) ? (int)v->valuedouble : cJSON_IsTrue(v); }
static double jnum(cJSON *o, const char *k) { cJSON *v = cJSON_GetObjectItemCaseSensitive(o, k); return cJSON_IsNumber(v) ? v->valuedouble : 0; }
static void cpy(char *d, size_t n, const char *s) { snprintf(d, n, "%s", s ? s : ""); }

static void set_status(const char *key, int n, const char *a0, const char *a1, const char *a2) {
    cpy(S.st_key, sizeof S.st_key, key); S.st_n = n;
    cpy(S.st_arg[0], 256, a0); cpy(S.st_arg[1], 256, a1); cpy(S.st_arg[2], 256, a2);
}
static const wchar_t *status_text(void) { const char *a[3] = {S.st_arg[0], S.st_arg[1], S.st_arg[2]}; return TF(S.st_key, S.st_n, a); }

static char *human(double n, char *buf, size_t cap) {
    const char *u[] = {"B", "KB", "MB", "GB", "TB"}; int i = 0;
    while (n >= 1024 && i < 4) { n /= 1024; i++; }
    if (i == 0) snprintf(buf, cap, "%d B", (int)n); else snprintf(buf, cap, "%.1f %s", n, u[i]);
    return buf;
}

static int keep_status;
static cJSON *parse_reply(const char *json, int *ok, char *err, size_t errn) {
    cJSON *r = cJSON_Parse(json);
    *ok = 0; err[0] = 0;
    if (!r) { cpy(err, errn, "bad reply"); return NULL; }
    *ok = cJSON_IsTrue(cJSON_GetObjectItemCaseSensitive(r, "ok"));
    if (!*ok) cpy(err, errn, jstr(r, "error"));
    return r;
}

/* ---- actions -------------------------------------------------------------------------------------- */
static void show_msg(const char *title, const char *text) {
    cpy(S.dlg_title, sizeof S.dlg_title, title);
    lines_free(&S.dlg_lines);
    char *t = xstrdup(text), *p = t, *q;
    while ((q = strchr(p, '\n'))) { *q = 0; lines_add(&S.dlg_lines, p, W2K_TEXT, 0, 0); p = q + 1; }
    lines_add(&S.dlg_lines, p, W2K_TEXT, 0, 0);
    free(t);
    S.modal = M_MSG; S.popup = P_NONE; S.dlg_scroll = 0;
}

static const char *node_name(int i) { static const char *n[3] = {"FM1", "FM2", "BM"}; return n[i]; }
static char *args_obj1(const char *k, const char *v) { cJSON *o = cJSON_CreateObject(); cJSON_AddStringToObject(o, k, v); char *s = cJSON_PrintUnformatted(o); cJSON_Delete(o); return s; }

static int cur_rid(int kind, int seq) { return (kind << 16) | (seq & 0xFFFF); }

static void do_reload(void) {
    if (S.mach < 0 || S.mach >= S.nmach) return;
    S.list_seq++;
    cJSON *o = cJSON_CreateObject();
    cJSON_AddStringToObject(o, "machine", S.machines[S.mach]); cJSON_AddStringToObject(o, "node", node_name(S.node));
    char *a = cJSON_PrintUnformatted(o); cJSON_Delete(o);
    if (!keep_status) set_status("st_reading", 2, S.machines[S.mach], node_name(S.node), "");
    S.h->call_async(cur_rid(RQ_LIST, S.list_seq), "ListModels", a);
    S.h->call_async(cur_rid(RQ_TARGETS, S.list_seq), "Targets", a);
    free(a);
}

static int ci_cmp(const char *a, const char *b) {
    for (;; a++, b++) {
        int x = (unsigned char)*a, y = (unsigned char)*b;
        if (x >= 'A' && x <= 'Z') x += 32;
        if (y >= 'A' && y <= 'Z') y += 32;
        if (x != y) return x - y;
        if (!x) return 0;
    }
}
static int ci_contains(const char *hay, const char *needle) {
    if (!*needle) return 1;
    for (; *hay; hay++) { const char *h = hay, *n = needle; while (*h && *n && ((*h | 32) == (*n | 32) || *h == *n)) { h++; n++; } if (!*n) return 1; }
    return 0;
}

static int row_cmp_col, row_cmp_desc;
static int row_cmp(const void *pa, const void *pb) {
    const Row *a = &S.rows[*(const int *)pa], *b = &S.rows[*(const int *)pb];
    int c = 0;
    if (row_cmp_col == 1) c = strcmp(a->cls, b->cls);
    else if (row_cmp_col == 6) c = strcmp(a->mod, b->mod);
    if (!c) c = ci_cmp(a->name, b->name);
    return row_cmp_desc ? -c : c;
}

/* utf-8 search text from the native EDIT */
static char *edit_utf8(int e) { return u8w(edit_cache[e]); }
static void refresh_edit(int e) { S.h->edit_get(e, edit_cache[e], EDIT_TEXTLEN); }

static void apply_filter(void) {
    char *q = edit_utf8(EDIT_SEARCH);
    char *keep = (S.sel_vis >= 0 && S.sel_vis < S.nvis) ? xstrdup(S.rows[S.vis[S.sel_vis]].name) : NULL;
    S.nvis = 0;
    S.vis = (int *)realloc(S.vis, (S.nrows + 1) * sizeof(int));
    for (int i = 0; i < S.nrows; i++) if (ci_contains(S.rows[i].name, q)) S.vis[S.nvis++] = i;
    row_cmp_col = S.sort_col; row_cmp_desc = S.sort_desc;
    qsort(S.vis, S.nvis, sizeof(int), row_cmp);
    S.sel_vis = -1;
    if (keep) for (int i = 0; i < S.nvis; i++) if (!ci_cmp(S.rows[S.vis[i]].name, keep)) S.sel_vis = i;
    free(keep); free(q);
    if (S.scroll > S.nvis) S.scroll = 0;
}

static void free_rows(void) {
    for (int i = 0; i < S.nrows; i++) { free(S.rows[i].name); free(S.rows[i].cls); free(S.rows[i].mod); }
    free(S.rows); S.rows = NULL; S.nrows = 0;
}

static void on_list(cJSON *res) {
    free_rows();
    cJSON *ms = cJSON_GetObjectItemCaseSensitive(res, "models");
    int n = cJSON_GetArraySize(ms);
    S.rows = (Row *)calloc(n + 1, sizeof(Row));
    for (int i = 0; i < n; i++) {
        cJSON *m = cJSON_GetArrayItem(ms, i); Row *r = &S.rows[S.nrows++];
        r->name = xstrdup(jstr(m, "name")); r->cls = xstrdup(jstr(m, "class")); r->mod = xstrdup(jstr(m, "modTime"));
        r->repo = jint(m, "repo"); r->his = jint(m, "history"); r->insp = jint(m, "inspect"); r->light = jint(m, "light");
    }
    S.read_only = jint(res, "readOnly"); cpy(S.path, sizeof S.path, jstr(res, "repository"));
    refresh_edit(EDIT_SEARCH); apply_filter();
    char cnt[16]; snprintf(cnt, sizeof cnt, "%d", S.nrows);
    if (keep_status) keep_status = 0; else set_status("st_read_ok", 3, S.machines[S.mach], node_name(S.node), cnt);
}

static void on_targets(cJSON *res) {
    for (int i = 0; i < S.nsingle; i++) { free(S.single[i].machine); free(S.single[i].node); }
    free(S.single); S.single = NULL; S.nsingle = 0;
    for (int i = 0; i < S.ngroup; i++) free(S.group[i]);
    free(S.group); S.group = NULL; S.ngroup = 0;
    cJSON *sg = cJSON_GetObjectItemCaseSensitive(res, "single"), *gr = cJSON_GetObjectItemCaseSensitive(res, "group");
    int n = cJSON_GetArraySize(sg);
    S.single = (Ref *)calloc(n + 1, sizeof(Ref));
    for (int i = 0; i < n; i++) { cJSON *r = cJSON_GetArrayItem(sg, i); S.single[S.nsingle].machine = xstrdup(jstr(r, "machine")); S.single[S.nsingle++].node = xstrdup(jstr(r, "node")); }
    n = cJSON_GetArraySize(gr);
    S.group = (char **)calloc(n + 1, sizeof(char *));
    for (int i = 0; i < n; i++) S.group[S.ngroup++] = xstrdup(cJSON_GetArrayItem(gr, i)->valuestring);
    S.tsingle = 0;
    for (int i = 0; i < S.nsingle; i++) if (!strcmp(S.single[i].machine, S.machines[S.mach]) && strcmp(S.single[i].node, node_name(S.node))) { S.tsingle = i; break; }
    S.tgroup = 0;
    if (!S.ngroup) S.mode_group = 0;
}

static void on_config(cJSON *res) {
    for (int i = 0; i < S.nmach; i++) free(S.machines[i]);
    free(S.machines); S.machines = NULL; S.nmach = 0;
    cJSON *ms = cJSON_GetObjectItemCaseSensitive(res, "machines");
    int n = cJSON_GetArraySize(ms);
    S.machines = (char **)calloc(n + 1, sizeof(char *));
    for (int i = 0; i < n; i++) S.machines[S.nmach++] = xstrdup(jstr(cJSON_GetArrayItem(ms, i), "name"));
    S.mach = 0; S.node = 0;
    do_reload();
}

static void build_plan_dialog(cJSON *plan) {
    lines_free(&S.dlg_lines);
    cpy(S.dlg_title, sizeof S.dlg_title, jstr(plan, "title"));
    cpy(S.plan_id, sizeof S.plan_id, jstr(plan, "id"));
    cJSON *bl = cJSON_GetObjectItemCaseSensitive(plan, "blockers"), *wa = cJSON_GetObjectItemCaseSensitive(plan, "warnings"), *un = cJSON_GetObjectItemCaseSensitive(plan, "units");
    char buf[1024];
    for (int i = 0; i < cJSON_GetArraySize(bl); i++) { const char *a[1] = {cJSON_GetArrayItem(bl, i)->valuestring}; lines_add(&S.dlg_lines, u8w(TF("plan_blocker", 1, a)) , W2K_NG, 1, 0); }
    for (int i = 0; i < cJSON_GetArraySize(wa); i++) { const char *a[1] = {cJSON_GetArrayItem(wa, i)->valuestring}; lines_add(&S.dlg_lines, u8w(TF("plan_warning", 1, a)), W2K_WARN, 0, 0); }
    for (int u = 0; u < cJSON_GetArraySize(un); u++) {
        cJSON *U = cJSON_GetArrayItem(un, u), *tg = cJSON_GetObjectItemCaseSensitive(U, "target"), *st = cJSON_GetObjectItemCaseSensitive(U, "steps");
        snprintf(buf, sizeof buf, "[%s / %s]  %s", jstr(tg, "machine"), jstr(tg, "node"), jstr(U, "model"));
        if (strcmp(jstr(U, "model"), jstr(U, "newName"))) { size_t l = strlen(buf); snprintf(buf + l, sizeof buf - l, " -> %s", jstr(U, "newName")); }
        lines_add(&S.dlg_lines, buf, W2K_TEXT, 1, 0);
        for (int k = 0; k < cJSON_GetArraySize(st); k++) {
            cJSON *s = cJSON_GetArrayItem(st, k);
            lines_add(&S.dlg_lines, jstr(s, "text"), cJSON_IsTrue(cJSON_GetObjectItemCaseSensitive(s, "danger")) ? W2K_NG : W2K_TEXT, 0, 1);
        }
    }
    S.dlg_can_run = cJSON_GetArraySize(bl) == 0;
    char hb[32]; const char *a[2] = {buf, hb};
    if (!strcmp(jstr(plan, "kind"), "copy")) { snprintf(buf, sizeof buf, "%d", jint(plan, "files")); human(jnum(plan, "bytes"), hb, sizeof hb); cpy(S.dlg_summary, sizeof S.dlg_summary, (char *)u8w(TF("plan_summary_copy", 2, a))); }
    else { snprintf(buf, sizeof buf, "%d", cJSON_GetArraySize(un)); cpy(S.dlg_summary, sizeof S.dlg_summary, (char *)u8w(TF("plan_summary_rename", 1, a))); }
    S.modal = M_PLAN; S.popup = P_NONE; S.dlg_scroll = 0;
}

static void start_job(void) {
    char *a = args_obj1("planId", S.plan_id);
    S.h->call_async(cur_rid(RQ_START, 0), "StartJob", a);
    free(a);
}

static void job_apply(cJSON *j) {
    cJSON *p = cJSON_GetObjectItemCaseSensitive(j, "progress"), *lg = cJSON_GetObjectItemCaseSensitive(p, "log");
    const char *state = jstr(j, "state");
    cpy(S.job_phase, sizeof S.job_phase, jstr(p, "phase")); cpy(S.job_current, sizeof S.job_current, jstr(p, "current"));
    char c0[32], c1[32], c2[64], c3[64], c4[32]; const char *a[5] = {c0, c1, c2, c3, c4};
    snprintf(c0, sizeof c0, "%d", jint(p, "filesDone")); snprintf(c1, sizeof c1, "%d", jint(p, "filesTotal"));
    human(jnum(p, "bytesDone"), c2, sizeof c2); human(jnum(p, "bytesTotal"), c3, sizeof c3); snprintf(c4, sizeof c4, "%.1f", jnum(p, "seconds"));
    { char *u = u8w(TF("job_counts", 5, a)); cpy(S.job_counts, sizeof S.job_counts, u); free(u); }
    S.job_pct = jnum(p, "bytesTotal") > 0 ? (int)(100 * jnum(p, "bytesDone") / jnum(p, "bytesTotal")) : jnum(p, "filesTotal") > 0 ? (int)(100 * jnum(p, "filesDone") / jnum(p, "filesTotal")) : 0;
    S.job_committed = jint(p, "committed");
    for (int i = S.job_log.n; i < cJSON_GetArraySize(lg); i++) lines_add(&S.job_log, cJSON_GetArrayItem(lg, i)->valuestring, W2K_TEXT, 0, 0);
    S.job_running = !strcmp(state, "running");
    if (!S.job_running) {
        S.job_done = 1; S.job_failed = strcmp(state, "done") != 0;
        if (!S.job_failed) { S.job_pct = 100; { char *u = u8w(T("job_done")); cpy(S.job_result, sizeof S.job_result, u); free(u); } }
        else cpy(S.job_result, sizeof S.job_result, jstr(j, "error")[0] ? jstr(j, "error") : state);
        set_status(S.job_failed ? (!strcmp(state, "canceled") ? "st_job_canceled" : "st_failed") : "st_done", 1, S.dlg_title, "", "");
        S.h->set_timer(0);
    }
}

static void begin_plan(const char *method, cJSON *req, int kind) {
    cJSON *o = cJSON_CreateObject();
    cJSON_AddItemToObject(o, "request", req);
    char *a = cJSON_PrintUnformatted(o); cJSON_Delete(o);
    S.pending_kind = kind; S.busy = 1;
    set_status("st_planning", 0, "", "", "");
    S.h->call_async(cur_rid(RQ_PLAN, 0), method, a);
    free(a);
}

static cJSON *ref_json(const char *m, const char *n) { cJSON *r = cJSON_CreateObject(); cJSON_AddStringToObject(r, "machine", m); cJSON_AddStringToObject(r, "node", n); return r; }

static void do_copy(void) {
    if (S.sel_vis < 0) return;
    refresh_edit(EDIT_COPYNAME);
    char *nn = edit_utf8(EDIT_COPYNAME);
    cJSON *r = cJSON_CreateObject();
    cJSON_AddStringToObject(r, "mode", S.mode_group ? "group" : "single");
    cJSON_AddStringToObject(r, "model", S.rows[S.vis[S.sel_vis]].name);
    cJSON_AddStringToObject(r, "newName", nn); free(nn);
    cJSON_AddItemToObject(r, "source", ref_json(S.machines[S.mach], node_name(S.node)));
    if (S.mode_group) { if (!S.ngroup) { cJSON_Delete(r); return; } cJSON_AddItemToObject(r, "target", ref_json(S.group[S.tgroup], node_name(S.node))); }
    else { if (!S.nsingle) { cJSON_Delete(r); return; } cJSON_AddItemToObject(r, "target", ref_json(S.single[S.tsingle].machine, S.single[S.tsingle].node)); }
    begin_plan("PlanCopy", r, 0);
}

static void do_rename(void) {
    if (S.sel_vis < 0) return;
    refresh_edit(EDIT_RENAME);
    char *nn = edit_utf8(EDIT_RENAME);
    cJSON *r = cJSON_CreateObject();
    cJSON_AddStringToObject(r, "mode", S.rename_group ? "group" : "single");
    cJSON_AddStringToObject(r, "model", S.rows[S.vis[S.sel_vis]].name);
    cJSON_AddStringToObject(r, "newName", nn); free(nn);
    cJSON_AddItemToObject(r, "ref", ref_json(S.machines[S.mach], node_name(S.node)));
    begin_plan("PlanRename", r, 1);
}

static void do_export(void) { S.h->call_async(cur_rid(RQ_EXPORT, 0), "ExportExcel", "{}"); }

void app_on_result(int id, const char *json) {
    int kind = id >> 16, seq = id & 0xFFFF, ok; char err[512];
    cJSON *r = parse_reply(json, &ok, err, sizeof err);
    cJSON *res = r ? cJSON_GetObjectItemCaseSensitive(r, "result") : NULL;
    switch (kind) {
    case RQ_HEALTH: if (ok) { S.lock_open = jint(res, "allowNetworkWrite"); cpy(S.version, sizeof S.version, jstr(res, "version")); } break;
    case RQ_CONFIG: if (ok) on_config(res); break;
    case RQ_LIST:
        if (seq != (S.list_seq & 0xFFFF)) break;
        if (ok) on_list(res); else { free_rows(); S.nvis = 0; set_status("st_read_fail", 3, S.machines[S.mach], node_name(S.node), err); }
        break;
    case RQ_TARGETS: if (ok && seq == (S.list_seq & 0xFFFF)) on_targets(res); break;
    case RQ_PLAN:
        S.busy = 0;
        if (ok) build_plan_dialog(res);
        else { set_status("st_cannot", 1, err, "", ""); char *t = u8w(T("dlg_cannot")); show_msg(t, err); free(t); }
        break;
    case RQ_START:
        if (ok) {
            cpy(S.job_id, sizeof S.job_id, jstr(res, "id")); lines_free(&S.job_log); S.job_running = 1; S.job_done = 0; S.job_failed = 0; S.job_result[0] = 0; S.job_pct = 0;
            S.job_phase[0] = S.job_current[0] = S.job_counts[0] = 0; S.modal = M_JOB; S.dlg_scroll = 0;
            job_apply(res); if (S.job_running) S.h->set_timer(150);
        } else { char *t = u8w(T("dlg_cannot_start")); show_msg(t, err); free(t); }
        break;
    case RQ_JOB:
        if (ok) { job_apply(res); } else { S.job_running = 0; S.job_done = 1; S.job_failed = 1; cpy(S.job_result, sizeof S.job_result, err); S.h->set_timer(0); }
        break;
    case RQ_EXPORT: {
        char *t = u8w(T("tab_export")), *n = u8w(T("export_note")); char buf[1024]; snprintf(buf, sizeof buf, "%s\n%s", ok ? "" : err, n);
        show_msg(t, buf); free(t); free(n); break; }
    }
    if (r) cJSON_Delete(r);
    S.h->invalidate();
}

/* ======================================================================================================
 * painting
 * ==================================================================================================== */
static const w2k_gfx *G;
static int th(void) { return w2k_th(G); }
static int tw(const wchar_t *s, int bold) { return w2k_tw(G, s, bold); }
static void txt(int x, int y, const wchar_t *s, uint32_t c, int bold) { w2k_txt(G, x, y, s, c, bold); }
static void fillr(int x, int y, int w, int h, uint32_t c) { w2k_fillr(G, x, y, w, h, c); }
static void hot_add(int x, int y, int w, int h, int act, int arg) { w2k_hot_add(&S.hot, x, y, w, h, act, arg); }
static int draw_wrapped(int x, int y, int w, const wchar_t *s, uint32_t c, int bold) { return w2k_draw_wrapped(G, x, y, w, s, c, bold); }
static int wrapped_h(int w, const wchar_t *s) { return w2k_wrapped_h(G, w, s); }

static void place_edit(int e, int x, int y, int w, int h, int visible) {
    w2k_draw_field(G, x, y, w, h);
    if (S.draw_edit_text) { G->clip(G->ctx, x + 3, y + 2, w - 6, h - 4); txt(x + 4, y + (h - th()) / 2, edit_cache[e], W2K_TEXT, 0); G->clip(G->ctx, 0, 0, 0, 0); }
    S.h->edit_place(e, x + 3, y + 3, w - 6, h - 6, visible);
}

static void radio_row(int x, int y, int checked, int act, const wchar_t *label, int disabled) {
    w2k_choice(G, &S.hot, x, y, 0, checked, 1, disabled, label, act, 0);
}

static void combo(int x, int y, int w, int h, const wchar_t *text, int cb, int disabled) { w2k_combo(G, &S.hot, x, y, w, h, text, A_COMBO, cb, disabled); }

static const wchar_t *combo_text(int cb, int i) {
    static wchar_t buf[300]; char tmp[300];
    switch (cb) {
    case CB_MACHINE: return i >= 0 && i < S.nmach ? w8(S.machines[i]) : L"";
    case CB_NODE: return w8(node_name(i));
    case CB_TSINGLE: if (i < 0 || i >= S.nsingle) return L""; snprintf(tmp, sizeof tmp, "%s / %s", S.single[i].machine, S.single[i].node); return w8(tmp);
    default: return i >= 0 && i < S.ngroup ? w8(S.group[i]) : L"";
    }
    (void)buf;
}
static int combo_count(int cb) { return cb == CB_MACHINE ? S.nmach : cb == CB_NODE ? 3 : cb == CB_TSINGLE ? S.nsingle : S.ngroup; }
static int combo_cur(int cb) { return cb == CB_MACHINE ? S.mach : cb == CB_NODE ? S.node : cb == CB_TSINGLE ? S.tsingle : S.tgroup; }

static const wchar_t *list_cell(void *ud, int vi, int col) {
    static wchar_t buf[2][160]; (void)ud;
    Row *R = &S.rows[S.vis[vi]];
    const wchar_t *yes = T("yes_mark"), *no = T("no_mark");
    switch (col) {
    case 0: return w8(R->name);
    case 1: return w8(R->cls);
    case 2: return R->repo ? yes : no;
    case 3: return R->his ? yes : no;
    case 4: if (R->insp) { swprintf(buf[0], 16, L"%d", R->insp); return buf[0]; } return no;
    case 5: return R->light ? yes : no;
    default: { char mod[32] = ""; if (strlen(R->mod) >= 16) snprintf(mod, sizeof mod, "%.10s %.5s", R->mod, R->mod + 11); return w8(mod); }
    }
}

static void paint_list(int x, int y, int w, int h) {
    static const int col_px[7] = {120, 56, 96, 44, 108, 88, 124};
    const wchar_t *hd[7] = {T("col_model"), T("col_class"), L"PxRepository", L".his", L"INSPECT_SPEC", L"LIGHT_SPEC", T("col_modified")};
    w2k_list L = {7, col_px, hd, S.nvis, list_cell, NULL, S.sel_vis, &S.scroll, A_HDR, A_ROW, A_SCROLL};
    w2k_list_paint(G, &S.hot, x, y, w, h, &L);
}

static int group_title_h(void) { return th() + w2k_px(6); }

static void paint_copy_page(int px, int py, int pw, int ph) {
    int pad = w2k_px(4), rw = w2k_px(340), gap = w2k_px(6);
    int lx = px, lw = pw - rw - gap, rx = px + lw + gap;
    int ch = th() + w2k_px(8);                                    /* control height */
    /* source group */
    int gh = group_title_h() + ch + w2k_px(12);
    w2k_draw_group(G, lx, py, lw, gh, T("gb_source"));
    int cy = py + group_title_h() + w2k_px(2), cx = lx + w2k_px(10);
    const wchar_t *l1 = T("lbl_machine"), *l2 = T("lbl_node");
    txt(cx, cy + (ch - th()) / 2, l1, W2K_TEXT, 0); cx += tw(l1, 0) + w2k_px(6);
    combo(cx, cy, w2k_px(150), ch, combo_text(CB_MACHINE, S.mach), CB_MACHINE, 0); cx += w2k_px(150) + w2k_px(14);
    txt(cx, cy + (ch - th()) / 2, l2, W2K_TEXT, 0); cx += tw(l2, 0) + w2k_px(6);
    combo(cx, cy, w2k_px(86), ch, combo_text(CB_NODE, S.node), CB_NODE, 0); cx += w2k_px(86) + w2k_px(14);
    int bw = w2k_px(80), ex = cx, ew = lx + lw - w2k_px(10) - bw - w2k_px(8) - ex;
    place_edit(EDIT_SEARCH, ex, cy, ew > w2k_px(60) ? ew : w2k_px(60), ch, !S.modal && !S.popup);
    w2k_button(G, &S.hot, lx + lw - w2k_px(10) - bw, cy, bw, ch, T("btn_refresh"), A_REFRESH, 0, S.pressed_hot == A_REFRESH ? W2K_PRESSED : 0);
    /* list */
    paint_list(lx, py + gh + w2k_px(6), lw, ph - gh - w2k_px(6));

    /* right column */
    int y = py, gw = rw;
    int in = rx + w2k_px(10), iw = gw - w2k_px(20);
    /* copy */
    int copy_top = y; y += group_title_h() + w2k_px(2);
    char src[300]; snprintf(src, sizeof src, "%s / %s", S.mach < S.nmach && S.nmach ? S.machines[S.mach] : "", node_name(S.node));
    { const char *a[1] = {src}; txt(in, y, TF("lbl_source_is", 1, a), W2K_TEXT_DIM, 0); } y += th() + w2k_px(6);
    radio_row(in, y, !S.mode_group, A_MODE_SINGLE, T("rb_single"), !S.nsingle); y += th() + w2k_px(6);
    combo(in + w2k_px(18), y, iw - w2k_px(18), ch, combo_text(CB_TSINGLE, S.tsingle), CB_TSINGLE, S.mode_group || !S.nsingle); y += ch + w2k_px(6);
    radio_row(in, y, S.mode_group, A_MODE_GROUP, T("rb_group"), !S.ngroup); y += th() + w2k_px(6);
    combo(in + w2k_px(18), y, iw - w2k_px(18), ch, combo_text(CB_TGROUP, S.tgroup), CB_TGROUP, !S.mode_group || !S.ngroup); y += ch + w2k_px(8);
    { const wchar_t *l = T("lbl_newname_opt"); txt(in, y + (ch - th()) / 2, l, W2K_TEXT, 0); int ex2 = in + tw(l, 0) + w2k_px(6);
      place_edit(EDIT_COPYNAME, ex2, y, in + iw - ex2, ch, !S.modal && !S.popup); } y += ch + w2k_px(6);
    y += draw_wrapped(in, y, iw, T("note_copy"), W2K_TEXT_DIM, 0) + w2k_px(6);
    int canCopy = S.sel_vis >= 0 && !S.busy && (S.mode_group ? S.ngroup : S.nsingle);
    w2k_button(G, &S.hot, in + iw - w2k_px(96), y, w2k_px(96), ch, T("btn_copy"), A_COPY, 0, (S.pressed_hot == A_COPY ? W2K_PRESSED : 0) | (canCopy ? 0 : W2K_DISABLED));
    y += ch + w2k_px(10);
    w2k_draw_group(G, rx, copy_top, gw, y - copy_top, T("gb_copy")); y += w2k_px(6);
    /* rename */
    int ren_top = y; y += group_title_h() + w2k_px(2);
    w2k_choice(G, &S.hot, in, y, iw, S.rename_group, 0, 0, T("chk_rename_group"), A_CHK_RENAME, 0); y += th() + w2k_px(8);
    { const wchar_t *l = T("lbl_newname"); txt(in, y + (ch - th()) / 2, l, W2K_TEXT, 0); int ex2 = in + tw(l, 0) + w2k_px(6);
      place_edit(EDIT_RENAME, ex2, y, in + iw - ex2, ch, !S.modal && !S.popup); } y += ch + w2k_px(8);
    int canRen = S.sel_vis >= 0 && !S.busy;
    w2k_button(G, &S.hot, in + iw - w2k_px(96), y, w2k_px(96), ch, T("btn_rename"), A_RENAME, 0, (S.pressed_hot == A_RENAME ? W2K_PRESSED : 0) | (canRen ? 0 : W2K_DISABLED));
    y += ch + w2k_px(10);
    w2k_draw_group(G, rx, ren_top, gw, y - ren_top, T("gb_rename")); y += w2k_px(6);
    /* rules */
    int rules_top = y; y += group_title_h() + w2k_px(2);
    const char *rk[4] = {"rule_1", "rule_2", "rule_3", "rule_4"};
    for (int i = 0; i < 4; i++) y += draw_wrapped(in, y, iw, T(rk[i]), W2K_TEXT_DIM, 0) + 2;
    y += w2k_px(6);
    w2k_draw_group(G, rx, rules_top, gw, y - rules_top, T("gb_rules"));
    (void)pad; (void)ph;
}

static void paint_export_page(int px, int py, int pw, int ph) {
    int w = w2k_px(520) < pw ? w2k_px(520) : pw, ch = th() + w2k_px(8);
    int tl = wrapped_h(w - w2k_px(20), T("export_page_text"));
    int h = group_title_h() + w2k_px(4) + tl + w2k_px(10) + ch + w2k_px(12);
    int y = py + group_title_h() + w2k_px(4);
    draw_wrapped(px + w2k_px(10), y, w - w2k_px(20), T("export_page_text"), W2K_TEXT, 0); y += tl + w2k_px(10);
    w2k_button(G, &S.hot, px + w2k_px(10), y, w2k_px(96), ch, T("btn_export"), A_EXPORT, 0, S.pressed_hot == A_EXPORT ? W2K_PRESSED : 0);
    w2k_draw_group(G, px, py, w, h, T("gb_export"));
    (void)ph;
    S.h->edit_place(EDIT_SEARCH, 0, 0, 0, 0, 0); S.h->edit_place(EDIT_COPYNAME, 0, 0, 0, 0, 0); S.h->edit_place(EDIT_RENAME, 0, 0, 0, 0, 0);
}

typedef struct { wchar_t text[96]; int code; int checked, sep; } MItem;
enum { MC_REFRESH = 1, MC_EXIT, MC_EXPORT, MC_ABOUT, MC_FONT = 100, MC_LANG = 200 };
static int menu_items(int m, MItem *o) {
    int n = 0;
    #define ADD(txtv, c, chk, sp) do { wcsncpy(o[n].text, txtv, 95); o[n].text[95] = 0; o[n].code = c; o[n].checked = chk; o[n].sep = sp; n++; } while (0)
    if (m == 0) { ADD(T("menu_refresh"), MC_REFRESH, 0, 0); ADD(L"", 0, 0, 1); ADD(T("menu_exit"), MC_EXIT, 0, 0); }
    else if (m == 1) {
        static const int sizes[6] = {12, 13, 14, 16, 18, 20};
        ADD(T("menu_fontsize"), 0, 0, 0);
        for (int i = 0; i < 6; i++) { wchar_t b[40]; swprintf(b, 40, L"   %d", sizes[i]); ADD(b, MC_FONT + sizes[i], sizes[i] == w2k_font_px, 0); }
        ADD(L"", 0, 0, 1); ADD(T("menu_language"), 0, 0, 0);
        for (int i = 0; i < i18n_count(); i++) { wchar_t b[60]; swprintf(b, 60, L"   %ls", w8(i18n_native_name(i))); ADD(b, MC_LANG + i, !strcmp(i18n_tag_at(i), i18n_tag()), 0); }
    }
    else if (m == 2) ADD(T("menu_export"), MC_EXPORT, 0, 0);
    else { ADD(T("menu_about"), MC_ABOUT, 0, 0); }
    #undef ADD
    return n;
}

static const wchar_t *combo_text_fn(void *ud, int i) { return combo_text(*(int *)ud, i); }

static void paint_popup(int W, int H) {
    if (S.popup == P_MENU) {
        MItem it[16]; w2k_menu_item mi[16]; int n = menu_items(S.popup_arg, it);
        for (int i = 0; i < n; i++) { mi[i].text = it[i].text; mi[i].code = it[i].code; mi[i].checked = it[i].checked; mi[i].sep = it[i].sep; }
        w2k_popup_menu(G, &S.hot, W, H, S.popup_x, S.popup_y, mi, n, A_POPITEM, A_NONE);
    } else if (S.popup == P_COMBO) {
        int cb = S.popup_arg;
        w2k_droplist(G, &S.hot, W, H, S.popup_x, S.popup_y, S.popup_w, combo_text_fn, &cb, combo_count(cb), combo_cur(cb), A_POPITEM, A_NONE);
    }
}

/* the dialog text: utf-8 lines -> wide copies (w8 buffers rotate) -> w2k_textview */
static void paint_text_area(int x, int y, int w, int h, const Lines *L, int *scroll, int stick_bottom) {
    w2k_line *ln = (w2k_line *)calloc(L->n + 1, sizeof(w2k_line));
    for (int i = 0; i < L->n; i++) {
        const wchar_t *s = w8(L->v[i].text); wchar_t *copy = (wchar_t *)malloc((wcslen(s) + 1) * sizeof(wchar_t)); wcscpy(copy, s);
        ln[i].text = copy; ln[i].rgb = L->v[i].color; ln[i].bold = L->v[i].bold; ln[i].indent = L->v[i].indent;
    }
    w2k_textview(G, x, y, w, h, ln, L->n, scroll, stick_bottom);
    for (int i = 0; i < L->n; i++) free((void *)ln[i].text);
    free(ln);
}

static void paint_modal(int W, int H) {
    int ch = th() + w2k_px(8), bw = w2k_px(96);
    int dw = W - 2 * w2k_px(30); if (dw > w2k_px(760)) dw = w2k_px(760);
    int dh = H - 2 * w2k_px(30); if (dh > w2k_px(480)) dh = w2k_px(480);
    if (S.modal == M_MSG) { dh = w2k_px(60) + (S.dlg_lines.n + 1) * (th() + 2) + ch + w2k_px(40); if (dh > H - 2 * w2k_px(30)) dh = H - 2 * w2k_px(30); if (dw > w2k_px(560)) dw = w2k_px(560); }
    int dx = (W - dw) / 2, dy = (H - dh) / 2;
    const wchar_t *title = S.modal == M_JOB ? (S.job_done ? T(S.job_failed ? "job_failed" : "job_done") : T("job_running")) : S.modal == M_PLAN ? T("plan_title") : T("dlg_error");
    if (S.modal == M_MSG && !strcmp(S.dlg_title, "")) title = L"";
    int capH = w2k_dialog_frame(G, dx, dy, dw, dh, S.modal == M_MSG ? w8(S.dlg_title) : title);
    int by = dy + dh - w2k_px(10) - ch, bx = dx + dw - w2k_px(14) - bw;
    int x = dx + w2k_px(14), w = dw - w2k_px(28), y = dy + 3 + capH + w2k_px(10);
    if (S.modal == M_PLAN) {
        txt(x, y, w8(S.dlg_title), W2K_TEXT, 1); y += th() + w2k_px(8);
        paint_text_area(x, y, w, by - w2k_px(10) - y, &S.dlg_lines, &S.dlg_scroll, 0);
        hot_add(x, y, w, by - w2k_px(10) - y, A_SCROLL, 1);
        w2k_button(G, &S.hot, bx, by, bw, ch, T("btn_cancel"), A_DLG_CANCEL, 0, 0);
        bx -= bw + w2k_px(8);
        w2k_button(G, &S.hot, bx, by, bw, ch, T("btn_run"), A_DLG_RUN, 0, W2K_DEFAULT | (S.dlg_can_run ? W2K_FOCUS : W2K_DISABLED));
        txt(x, by + (ch - th()) / 2, w8(S.dlg_summary), W2K_TEXT_DIM, 0);
    } else if (S.modal == M_JOB) {
        txt(x, y, w8(S.dlg_title), W2K_TEXT, 1); y += th() + w2k_px(8);
        txt(x, y, w8(S.job_phase), W2K_TEXT, 0); y += th() + w2k_px(6);
        w2k_draw_progress(G, x, y, w, w2k_px(18), S.job_pct); y += w2k_px(18) + w2k_px(6);
        txt(x, y, w8(S.job_counts), W2K_TEXT_DIM, 0); y += th() + 2;
        G->clip(G->ctx, x, y, w, th() + 2); txt(x, y, w8(S.job_current), W2K_TEXT_DIM, 0); G->clip(G->ctx, 0, 0, 0, 0); y += th() + w2k_px(8);
        int resh = S.job_result[0] ? th() + w2k_px(6) : 0;
        paint_text_area(x, y, w, by - w2k_px(10) - resh - y, &S.job_log, &S.dlg_scroll, 1);
        if (S.job_result[0]) txt(x, by - w2k_px(6) - resh + w2k_px(2), w8(S.job_result), S.job_failed ? W2K_NG : W2K_TEXT, 1);
        w2k_button(G, &S.hot, bx, by, bw, ch, T("btn_close"), A_DLG_CLOSE, 0, W2K_DEFAULT | (S.job_running ? W2K_DISABLED : W2K_FOCUS));
        bx -= bw + w2k_px(8);
        if (S.job_running) w2k_button(G, &S.hot, bx, by, bw, ch, T("btn_cancel_c"), A_DLG_CANCEL, 1, S.job_committed ? W2K_DISABLED : 0);
    } else {
        for (int i = 0; i < S.dlg_lines.n; i++) { int h = draw_wrapped(x, y, w, w8(S.dlg_lines.v[i].text), W2K_TEXT, 0); y += h + 1; }
        w2k_button(G, &S.hot, dx + (dw - bw) / 2, by, bw, ch, L"OK", A_DLG_OK, 0, W2K_DEFAULT | W2K_FOCUS);
    }
    hot_add(0, 0, W, H, A_NONE, -2);                  /* modal: swallow everything else */
}

const char *app_caption_text(void) { static char b[96]; snprintf(b, sizeof b, "AFVI TOOLS %s", S.version); return b; }

/* ---- page painting ------------------------------------------------------------------------------ */
void app_paint(w2k_gfx *g, int W, int H) {
    G = g; S.W = W; S.H = H; w2k_hot_reset(&S.hot);
    fillr(0, 0, W, H, W2K_FACE);
    int y = 0, m = w2k_px(4);
    if (S.frameless) {
        y = w2k_window_caption(G, &S.hot, W, H, w8(app_caption_text()), A_CAP, S.pressed_hot);
    }
    /* menu bar */
    int mh = th() + w2k_px(6);
    const wchar_t *ml[4] = {T("menu_file"), T("menu_view"), T("menu_tools"), T("menu_help")};
    w2k_menubar(G, &S.hot, m + 2, y, mh, ml, 4, S.popup == P_MENU ? S.popup_arg : -1, A_MENU);
    y += mh + w2k_px(2);
    /* tabs and the page frame */
    int tabH = th() + w2k_px(10);
    const wchar_t *tl[2] = {T("tab_copy"), T("tab_export")};
    int sbH = w2k_px(24), pageBot = H - sbH - 2 - (S.frameless ? 3 : 0);
    int pw = W - 2 * m - (S.frameless ? 6 : 0), px0 = m + (S.frameless ? 3 : 0);
    int pageTop = w2k_tabsheet(G, &S.hot, m + 10, y, px0, pw, pageBot, tl, 2, S.page, A_TAB0);
    (void)tabH;
    int in = w2k_px(8);
    if (S.page == 0) paint_copy_page(px0 + in, pageTop + in, pw - 2 * in, pageBot - pageTop - 2 * in);
    else paint_export_page(px0 + in, pageTop + in, pw - 2 * in, pageBot - pageTop - 2 * in);
    /* status bar: 4 sunken panes, the first takes the rest, size grip */
    int sy = H - sbH - (S.frameless ? 3 : 0);
    const wchar_t *p1 = status_text(), *p2 = w8(S.path);
    wchar_t cnt[96];
    { char a[16], b[16]; snprintf(a, sizeof a, "%d", S.nvis); snprintf(b, sizeof b, "%d", S.nrows); const char *args[2] = {a, b};
      char *q = edit_utf8(EDIT_SEARCH); int filt = q[0]; free(q);
      wcsncpy(cnt, TF(filt ? "count_filtered" : "count_models", filt ? 2 : 1, filt ? args : &args[1]), 95); cnt[95] = 0; }
    const wchar_t *p4 = S.read_only ? T(S.lock_open ? "lock_open" : "lock_locked") : T("lock_writable");
    const wchar_t *panes[4] = {p1, p2, cnt, p4};
    w2k_statusbar(G, px0, sy, pw, sbH, panes, 4);
    if (S.popup != P_NONE) paint_popup(W, H);
    if (S.modal != M_NONE) { S.h->edit_place(EDIT_SEARCH, 0, 0, 0, 0, 0); S.h->edit_place(EDIT_COPYNAME, 0, 0, 0, 0, 0); S.h->edit_place(EDIT_RENAME, 0, 0, 0, 0, 0); paint_modal(W, H); }
}

/* ---- input ------------------------------------------------------------------------------------------------- */
static int page_rows(void) { int rh = th() + 3, hh = th() + w2k_px(6); int h = S.H - w2k_px(300); (void)hh; return h / rh > 5 ? h / rh : 5; }
static void select_vis(int i) { w2k_list_select(&S.sel_vis, &S.scroll, S.nvis, i, page_rows()); }

static void do_menu_code(int code) {
    if (code == MC_REFRESH) { refresh_edit(EDIT_SEARCH); do_reload(); }
    else if (code == MC_EXIT) S.h->quit(0);
    else if (code == MC_EXPORT) do_export();
    else if (code == MC_ABOUT) { const char *a[1] = {S.version}; char *t = u8w(T("about_title")), *x = u8w(TF("about_text", 1, a)); show_msg(t, x); free(t); free(x); }
    else if (code >= MC_LANG) app_set_lang(i18n_tag_at(code - MC_LANG));
    else if (code >= MC_FONT) app_set_font(code - MC_FONT);
}

static void perform(int hi) {
    w2k_hot hot = S.hot.v[hi]; int act = hot.act, arg = hot.arg;
    switch (act) {
    case A_TAB0: S.page = 0; break;
    case A_TAB1: S.page = 1; break;
    case A_REFRESH: refresh_edit(EDIT_SEARCH); do_reload(); break;
    case A_COPY: do_copy(); break;
    case A_RENAME: do_rename(); break;
    case A_EXPORT: do_export(); break;
    case A_MODE_SINGLE: if (S.nsingle) S.mode_group = 0; break;
    case A_MODE_GROUP: if (S.ngroup) S.mode_group = 1; break;
    case A_CHK_RENAME: S.rename_group = !S.rename_group; break;
    case A_COMBO: S.popup = P_COMBO; S.popup_arg = arg; S.popup_x = hot.x; S.popup_y = hot.y + hot.h; S.popup_w = hot.w; break;
    case A_MENU: S.popup = P_MENU; S.popup_arg = arg; S.popup_x = hot.x; S.popup_y = hot.y + hot.h; break;
    case A_POPITEM:
        if (S.popup == P_COMBO) {
            int cb = S.popup_arg; S.popup = P_NONE;
            if (cb == CB_MACHINE) { S.mach = arg; S.sel_vis = -1; do_reload(); }
            else if (cb == CB_NODE) { S.node = arg; S.sel_vis = -1; do_reload(); }
            else if (cb == CB_TSINGLE) S.tsingle = arg; else S.tgroup = arg;
        } else { int code = arg; S.popup = P_NONE; do_menu_code(code); }
        break;
    case A_NONE: if (arg == -1) S.popup = P_NONE; break;
    case A_CAP: S.h->window_cmd(arg); break;
    case A_HDR: if (arg == 0 || arg == 1 || arg == 6) { if (S.sort_col == arg) S.sort_desc = !S.sort_desc; else { S.sort_col = arg; S.sort_desc = 0; } apply_filter(); } break;
    case A_DLG_RUN: start_job(); break;
    case A_DLG_CANCEL:
        if (arg == 1) { char *a = args_obj1("id", S.job_id); char *r = S.h->call_sync("CancelJob", a); free(a); free(r); }
        else { S.modal = M_NONE; set_status("st_canceled", 0, "", "", ""); }
        break;
    case A_DLG_CLOSE: S.modal = M_NONE; keep_status = 1; refresh_edit(EDIT_SEARCH); do_reload(); break;
    case A_DLG_OK: S.modal = M_NONE; break;
    default: break;
    }
}

static int hit(int x, int y) { return w2k_hot_at(&S.hot, x, y); }
static int down_hot = -1, down_act = 0;

void app_mouse(int kind, int x, int y) {
    int i = hit(x, y);
    if (kind == 0) {
        down_hot = i; S.pressed_hot = 0;
        if (i < 0) return;
        w2k_hot *t = &S.hot.v[i]; down_act = t->act;
        if (t->act == A_ROW) { select_vis(t->arg); S.h->invalidate(); down_hot = -1; return; }
        if (t->act == A_SCROLL && t->arg == 0) { int page = page_rows(); S.scroll += (y < t->y + t->h / 2) ? -page : page; S.h->invalidate(); down_hot = -1; return; }
        if (t->act == A_REFRESH || t->act == A_COPY || t->act == A_RENAME || t->act == A_EXPORT) S.pressed_hot = t->act;
        if (t->act == A_CAP) S.pressed_hot = A_CAP * 10 + t->arg;
        S.h->invalidate();
    } else if (kind == 1) {
        int d = down_hot; S.pressed_hot = 0; down_hot = -1;
        if (d >= 0 && d < S.hot.n && i >= 0 && i == d) perform(d);
        S.h->invalidate();
    }
}

void app_wheel(int x, int y, int delta) {
    (void)x; (void)y;
    if (S.modal == M_PLAN || S.modal == M_JOB || S.modal == M_MSG) S.dlg_scroll -= delta * 3; else S.scroll -= delta * 3;
    S.h->invalidate();
}

void app_key(int vk) {
    if (S.modal != M_NONE) {
        if (vk == VK_ESCAPE) { if (S.modal == M_PLAN) { S.modal = M_NONE; set_status("st_canceled", 0, "", "", ""); } else if (S.modal == M_MSG) S.modal = M_NONE; else if (S.modal == M_JOB && S.job_done) { S.modal = M_NONE; do_reload(); } }
        else if (vk == VK_RETURN) { if (S.modal == M_PLAN && S.dlg_can_run) start_job(); else if (S.modal == M_MSG) S.modal = M_NONE; else if (S.modal == M_JOB && S.job_done) { S.modal = M_NONE; do_reload(); } }
        S.h->invalidate(); return;
    }
    if (S.popup != P_NONE) { if (vk == VK_ESCAPE) S.popup = P_NONE; S.h->invalidate(); return; }
    if (vk == VK_F5) { refresh_edit(EDIT_SEARCH); do_reload(); }
    else if (S.page == 0 && S.nvis) {
        int cur = S.sel_vis < 0 ? 0 : S.sel_vis, pg = page_rows();
        if (vk == VK_DOWN) select_vis(S.sel_vis < 0 ? 0 : cur + 1); else if (vk == VK_UP) select_vis(cur - 1);
        else if (vk == VK_NEXT) select_vis(cur + pg); else if (vk == VK_PRIOR) select_vis(cur - pg);
        else if (vk == VK_HOME) select_vis(0); else if (vk == VK_END) select_vis(S.nvis - 1);
    }
    S.h->invalidate();
}

void app_edit_changed(int e) { refresh_edit(e); if (e == EDIT_SEARCH) apply_filter(); S.h->invalidate(); }

int app_hit_caption(int x, int y) {
    if (!S.frameless) return 0;
    int capH = th() + w2k_px(6);
    if (y < 3 || y >= 3 + capH) return 0;
    for (int i = 0; i < S.hot.n; i++) if (S.hot.v[i].act == A_CAP && x >= S.hot.v[i].x && x < S.hot.v[i].x + S.hot.v[i].w) return 0;
    return 1;
}

void app_set_lang(const char *tag) { i18n_set(tag); S.h->set_font(i18n_font_face(), w2k_font_px); S.h->invalidate(); }
void app_set_font(int px) { if (px < 10) px = 10; if (px > 28) px = 28; w2k_font_px = px; S.h->set_font(i18n_font_face(), px); S.h->invalidate(); }

/* ---- self-test: drives the same actions a user would, then verifies the files through the core ------------- */
static int st_fail(const char *msg) { snprintf(S.st_msg, sizeof S.st_msg, "%.250s", msg); return 1; }
static int find_row(const char *name) { for (int i = 0; i < S.nvis; i++) if (!ci_cmp(S.rows[S.vis[i]].name, name)) return i; return -1; }
static int verify(const char *step) {
    char *a = args_obj1("step", step), *r = S.h->call_sync("SelftestVerify", a); free(a);
    int ok; char err[256]; cJSON *j = parse_reply(r, &ok, err, sizeof err);
    int good = ok && cJSON_IsString(cJSON_GetObjectItemCaseSensitive(j, "result")) && !jstr(j, "result")[0];
    if (!good) snprintf(S.st_msg, sizeof S.st_msg, "verify %s: %s", step, r);
    if (j) cJSON_Delete(j);
    free(r);
    return good;
}

static void st_finish(int code) {
    char p[600]; snprintf(p, sizeof p, "%s/selftest-result.txt", S.st_dir);
    FILE *f = fopen(p, "w"); if (f) { fprintf(f, "%s %s\n", code ? "SELFTEST FAIL" : "SELFTEST OK", S.st_msg); fclose(f); }
    S.h->quit(code);
}

static void selftest_tick(void) {
    if (++S.st_wait > 400) { snprintf(S.st_msg, sizeof S.st_msg, "timeout in step %d", S.st_step); st_finish(1); return; }
    switch (S.st_step) {
    case 0: if (S.nmach == 2 && S.nrows == 3 && !S.busy) { S.st_step = 1; S.st_wait = 0; } break;
    case 1: { /* search is case-insensitive */
        S.h->edit_set(EDIT_SEARCH, L"m99"); app_edit_changed(EDIT_SEARCH);
        if (S.nvis != 1 || ci_cmp(S.rows[S.vis[0]].name, "7M99-535")) { st_fail("search"); st_finish(1); return; }
        S.h->edit_set(EDIT_SEARCH, L""); app_edit_changed(EDIT_SEARCH);
        if (S.nvis != 3) { st_fail("search cleared"); st_finish(1); return; }
        S.st_step = 2; S.st_wait = 0; break; }
    case 2: { /* four languages, all different, none shows a key */
        wchar_t seen[4][64];
        for (int i = 0; i < 4; i++) {
            app_set_lang(i18n_tag_at(i)); wcsncpy(seen[i], T("tab_copy"), 63); seen[i][63] = 0;
            if (!wcscmp(seen[i], L"tab_copy")) { st_fail("key shown"); st_finish(1); return; }
            for (int k = 0; k < i; k++) if (!wcscmp(seen[i], seen[k])) { st_fail("languages equal"); st_finish(1); return; }
        }
        app_set_lang("en"); S.st_step = 3; S.st_wait = 0; break; }
    case 3: { /* no BM offered to an FM; copy m1 FM1 -> FM2 (replaces M1) */
        for (int i = 0; i < S.nsingle; i++) if (!strcmp(S.single[i].node, "BM")) { st_fail("BM offered"); st_finish(1); return; }
        int r = find_row("m1"); int t = -1;
        for (int i = 0; i < S.nsingle; i++) if (!strcmp(S.single[i].machine, "T1") && !strcmp(S.single[i].node, "FM2")) t = i;
        if (r < 0 || t < 0) { st_fail("m1 or target missing"); st_finish(1); return; }
        select_vis(r); S.tsingle = t; S.mode_group = 0; do_copy(); S.st_step = 4; S.st_wait = 0; break; }
    case 4: if (S.modal == M_PLAN) {
            int red = 0; for (int i = 0; i < S.dlg_lines.n; i++) if (S.dlg_lines.v[i].color == W2K_NG) red = 1;
            if (!red || !S.dlg_can_run) { st_fail("plan has no red deletion step"); st_finish(1); return; }
            start_job(); S.st_step = 5; S.st_wait = 0; } break;
    case 5: if (S.modal == M_JOB && S.job_done) {
            if (S.job_failed) { st_fail(S.job_result); st_finish(1); return; }
            if (!verify("copy")) { st_finish(1); return; }
            S.modal = M_NONE; keep_status = 1; do_reload(); S.st_step = 6; S.st_wait = 0; } break;
    case 6: if (!S.busy && S.nrows == 3 && find_row("m2") >= 0 && S.list_seq && !S.job_running) {
            select_vis(find_row("m2")); S.rename_group = 1; S.h->edit_set(EDIT_RENAME, L"r2"); refresh_edit(EDIT_RENAME); do_rename(); S.st_step = 7; S.st_wait = 0; } break;
    case 7: if (S.modal == M_PLAN) { start_job(); S.st_step = 8; S.st_wait = 0; } break;
    case 8: if (S.modal == M_JOB && S.job_done) {
            if (S.job_failed) { st_fail(S.job_result); st_finish(1); return; }
            if (!verify("rename")) { st_finish(1); return; }
            S.modal = M_NONE; do_reload(); S.st_step = 9; S.st_wait = 0; } break;
    case 9: if (find_row("r2") >= 0 && !S.busy) {
            select_vis(find_row("r2")); S.rename_group = 0; S.h->edit_set(EDIT_RENAME, L"bad/name"); refresh_edit(EDIT_RENAME); do_rename(); S.st_step = 10; S.st_wait = 0; } break;
    case 10: if (S.modal == M_MSG) {
            int found = 0; for (int i = 0; i < S.dlg_lines.n; i++) if (strstr(S.dlg_lines.v[i].text, "illegal")) found = 1;
            if (!found) { st_fail("refusal text"); st_finish(1); return; }
            S.modal = M_NONE; do_export(); S.st_step = 11; S.st_wait = 0; } break;
    case 11: if (S.modal == M_MSG) { S.modal = M_NONE; snprintf(S.st_msg, sizeof S.st_msg, "list,search,languages,copy,rename,refusal,excel"); st_finish(0); } break;
    }
}

void app_tick(void) {
    if (S.job_running && S.modal == M_JOB) { char *a = args_obj1("id", S.job_id); S.h->call_async(cur_rid(RQ_JOB, 0), "GetJob", a); free(a); }
    if (S.st_active) selftest_tick();
    S.h->invalidate();
}

void app_enable_selftest(const char *dir) { S.st_active = 1; cpy(S.st_dir, sizeof S.st_dir, dir); S.h->set_timer(100); }

void app_init(host_t *h, int frameless, const char *lang, int font_px) {
    S.h = h; S.frameless = frameless; w2k_font_px = font_px; S.sel_vis = -1;
    i18n_init(); i18n_set(lang);
    set_status("st_ready", 0, "", "", "");
    h->set_font(i18n_font_face(), w2k_font_px);
    h->call_async(cur_rid(RQ_HEALTH, 0), "Health", "{}");
    h->call_async(cur_rid(RQ_CONFIG, 0), "GetConfig", "{}");
}

/* Preview/self-test hook: drives the real actions from the Linux preview program.
 * 1 copy plan, 2 run the plan, 3 View menu, 4 node combo, 5 refused rename, 6 close dialog, 7 export entry, 8 select row 0, 9 group mode */
void app_preview_state(int what) {
    S.draw_edit_text = 1;
    switch (what) {
    case 1: select_vis(S.nvis > 1 ? 1 : 0); do_copy(); break;
    case 2: start_job(); break;
    case 3: S.popup = P_MENU; S.popup_arg = 1; S.popup_x = w2k_px(60); S.popup_y = (S.frameless ? 3 + th() + w2k_px(6) : 0) + th() + w2k_px(6); break;
    case 4: S.popup = P_COMBO; S.popup_arg = CB_NODE; S.popup_x = w2k_px(300); S.popup_y = w2k_px(150); S.popup_w = w2k_px(86); break;
    case 5: select_vis(0); S.h->edit_set(EDIT_RENAME, L"bad/name"); refresh_edit(EDIT_RENAME); do_rename(); break;
    case 6: S.modal = M_NONE; S.popup = P_NONE; break;
    case 7: do_export(); break;
    case 8: select_vis(0); break;
    case 9: if (S.ngroup) S.mode_group = 1; break;
    }
}
