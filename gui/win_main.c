/* Windows host: plain Win32 (UTF-16 W API), GDI double buffering, three native EDIT controls, worker threads for core calls.
 * No comctl32 v6 manifest, so no visual styles: nothing here depends on the system theme. */
#ifndef UNICODE
#define UNICODE
#endif
#ifndef _UNICODE
#define _UNICODE
#endif
#include <windows.h>
#include <shellapi.h>
#include <stdio.h>
#include "app.h"
#include "w2k.h"
#include "w2k_gdi.h"
#include "util.h"
#include "afvi_core.h"

#define WM_RESULT (WM_APP + 1)

static HWND g_hwnd, g_edit[EDIT_COUNT];
static int g_aa;
static HBRUSH g_white;
static int g_frameless;

/* ---- host callbacks ---------------------------------------------------------------------------------------- */
typedef struct { int id; char *method, *args; char lang[16]; } Job;
static DWORD WINAPI worker(LPVOID p) {
    Job *j = (Job *)p;
    char *r = afvi_call(j->method, j->lang, j->args);
    PostMessageW(g_hwnd, WM_RESULT, (WPARAM)j->id, (LPARAM)xstrdup(r));
    afvi_free(r); free(j->method); free(j->args); free(j);
    return 0;
}
#include "i18n.h"
static void h_call_async(int id, const char *method, const char *args) {
    Job *j = (Job *)calloc(1, sizeof *j);
    j->id = id; j->method = xstrdup(method); j->args = xstrdup(args ? args : "{}"); snprintf(j->lang, sizeof j->lang, "%s", i18n_tag());
    HANDLE t = CreateThread(NULL, 0, worker, j, 0, NULL);
    if (t) CloseHandle(t);
}
static char *h_call_sync(const char *method, const char *args) { char *r = afvi_call(method, i18n_tag(), args ? args : "{}"); char *c = xstrdup(r); afvi_free(r); return c; }
static void h_edit_place(int e, int x, int y, int w, int h, int visible) {
    if (!g_edit[e]) return;
    if (!visible || w <= 0) { if (IsWindowVisible(g_edit[e])) ShowWindow(g_edit[e], SW_HIDE); return; }
    RECT cur; GetWindowRect(g_edit[e], &cur); POINT p = {cur.left, cur.top}; ScreenToClient(g_hwnd, &p);
    if (p.x != x || p.y != y || cur.right - cur.left != w || cur.bottom - cur.top != h) MoveWindow(g_edit[e], x, y, w, h, TRUE);
    if (!IsWindowVisible(g_edit[e])) ShowWindow(g_edit[e], SW_SHOW);
}
static void h_edit_get(int e, wchar_t *buf, int n) { GetWindowTextW(g_edit[e], buf, n); }
static void h_edit_set(int e, const wchar_t *s) { SetWindowTextW(g_edit[e], s); }
static void h_invalidate(void) { InvalidateRect(g_hwnd, NULL, FALSE); }
static void h_window_cmd(int c) {
    if (c == 0) ShowWindow(g_hwnd, SW_MINIMIZE);
    else if (c == 1) ShowWindow(g_hwnd, IsZoomed(g_hwnd) ? SW_RESTORE : SW_MAXIMIZE);
    else PostMessageW(g_hwnd, WM_CLOSE, 0, 0);
}
static void h_set_timer(int ms) { if (ms > 0) SetTimer(g_hwnd, 1, ms, NULL); else KillTimer(g_hwnd, 1); }
static void h_set_font(const char *face, int px) {
    int en = !strcmp(face, "Tahoma");
    HDC dc = GetDC(g_hwnd); w2k_gdi_set_font(dc, (const wchar_t *)w8(face), px, en && g_aa == 0); ReleaseDC(g_hwnd, dc);
    for (int e = 0; e < EDIT_COUNT; e++) if (g_edit[e]) SendMessageW(g_edit[e], WM_SETFONT, (WPARAM)w2k_gdi_font(0), TRUE);
}
static void h_quit(int code) { PostQuitMessage(code); }

static host_t HOST = {h_call_async, h_call_sync, h_edit_place, h_edit_get, h_edit_set, h_invalidate, h_window_cmd, h_set_timer, h_set_font, h_quit};

/* ---- window procedure --------------------------------------------------------------------------------------- */
static void paint(HWND hw) {
    PAINTSTRUCT ps; HDC dc = BeginPaint(hw, &ps);
    RECT rc; GetClientRect(hw, &rc); int W = rc.right, H = rc.bottom;
    HDC mem = CreateCompatibleDC(dc); HBITMAP bmp = CreateCompatibleBitmap(dc, W, H); HGDIOBJ old = SelectObject(mem, bmp);
    w2k_gfx g = w2k_gdi_gfx(mem);
    app_paint(&g, W, H);
    BitBlt(dc, 0, 0, W, H, mem, 0, 0, SRCCOPY);
    SelectObject(mem, old); DeleteObject(bmp); DeleteDC(mem); EndPaint(hw, &ps);
}

static LRESULT CALLBACK wndproc(HWND hw, UINT m, WPARAM wp, LPARAM lp) {
    switch (m) {
    case WM_NCCALCSIZE: if (g_frameless && wp) return 0; break;                  /* no system frame: the skin draws it */
    case WM_NCHITTEST:
        if (g_frameless) {
            POINT p = {(short)LOWORD(lp), (short)HIWORD(lp)}; ScreenToClient(hw, &p); RECT r; GetClientRect(hw, &r); int b = 6;
            int L = p.x < b, R = p.x >= r.right - b, T_ = p.y < b, B = p.y >= r.bottom - b;
            if (!IsZoomed(hw)) {
                if (T_ && L) return HTTOPLEFT;
                if (T_ && R) return HTTOPRIGHT;
                if (B && L) return HTBOTTOMLEFT;
                if (B && R) return HTBOTTOMRIGHT;
                if (L) return HTLEFT;
                if (R) return HTRIGHT;
                if (T_) return HTTOP;
                if (B) return HTBOTTOM;
            }
            if (app_hit_caption(p.x, p.y)) return HTCAPTION;
            return HTCLIENT;
        }
        break;
    case WM_GETMINMAXINFO: { MINMAXINFO *mi = (MINMAXINFO *)lp; mi->ptMinTrackSize.x = w2k_px(900); mi->ptMinTrackSize.y = w2k_px(560); return 0; }
    case WM_PAINT: paint(hw); return 0;
    case WM_ERASEBKGND: return 1;
    case WM_SIZE: InvalidateRect(hw, NULL, FALSE); return 0;
    case WM_LBUTTONDOWN: SetFocus(hw); SetCapture(hw); app_mouse(0, (short)LOWORD(lp), (short)HIWORD(lp)); return 0;
    case WM_LBUTTONUP: ReleaseCapture(); app_mouse(1, (short)LOWORD(lp), (short)HIWORD(lp)); return 0;
    case WM_MOUSEMOVE: app_mouse(2, (short)LOWORD(lp), (short)HIWORD(lp)); return 0;
    case WM_MOUSEWHEEL: app_wheel(0, 0, (short)HIWORD(wp) / WHEEL_DELTA); return 0;
    case WM_KEYDOWN: app_key((int)wp); return 0;
    case WM_TIMER: app_tick(); return 0;
    case WM_RESULT: { char *s = (char *)lp; app_on_result((int)wp, s); free(s); return 0; }
    case WM_COMMAND: if (HIWORD(wp) == EN_CHANGE) for (int e = 0; e < EDIT_COUNT; e++) if ((HWND)lp == g_edit[e]) app_edit_changed(e); return 0;
    case WM_CTLCOLOREDIT: SetBkColor((HDC)wp, RGB(255, 255, 255)); SetTextColor((HDC)wp, RGB(0, 0, 0)); return (LRESULT)g_white;
    case WM_DESTROY: PostQuitMessage(0); return 0;
    }
    return DefWindowProcW(hw, m, wp, lp);
}

int WINAPI wWinMain(HINSTANCE hi, HINSTANCE prev, LPWSTR cmd, int show) {
    (void)prev; (void)cmd;
    int argc; LPWSTR *argv = CommandLineToArgvW(GetCommandLineW(), &argc);
    const char *config = NULL, *lang = "ko", *selftest = NULL; int font = 14;
    for (int i = 1; i < argc; i++) {
        char *a = u8w(argv[i]);
        if (!strcmp(a, "-frameless")) g_frameless = 1;
        else if (!strcmp(a, "-aa")) g_aa = 1;
        else if (i + 1 < argc && !strcmp(a, "-config")) config = u8w(argv[++i]);
        else if (i + 1 < argc && !strcmp(a, "-lang")) lang = u8w(argv[++i]);
        else if (i + 1 < argc && !strcmp(a, "-font")) font = _wtoi(argv[++i]);
        else if (i + 1 < argc && !strcmp(a, "-selftest")) selftest = u8w(argv[++i]);
        free(a);
    }
    SetProcessDPIAware();                                   /* classic API, works from Windows Vista on */
    HDC sdc = GetDC(NULL); int dpi = GetDeviceCaps(sdc, LOGPIXELSY); ReleaseDC(NULL, sdc);
    if (afvi_init(config, selftest, 0) != 0) { MessageBoxW(NULL, L"afvi core: init failed", L"AFVI TOOLS", MB_ICONERROR); return 1; }
    if (selftest) lang = "en";
    g_white = CreateSolidBrush(RGB(255, 255, 255));
    WNDCLASSEXW wc = {sizeof wc};
    wc.lpfnWndProc = wndproc; wc.hInstance = hi; wc.hCursor = LoadCursor(NULL, IDC_ARROW); wc.lpszClassName = L"AfviWin2000"; wc.style = CS_DBLCLKS;
    RegisterClassExW(&wc);
    DWORD style = g_frameless ? (WS_POPUP | WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX | WS_SYSMENU) : WS_OVERLAPPEDWINDOW;
    int px = font * dpi / 96;
    g_hwnd = CreateWindowExW(0, L"AfviWin2000", L"AFVI TOOLS", style, CW_USEDEFAULT, CW_USEDEFAULT, 1100 * px / 14, 720 * px / 14, NULL, NULL, hi, NULL);
    for (int e = 0; e < EDIT_COUNT; e++) g_edit[e] = CreateWindowExW(0, L"EDIT", L"", WS_CHILD | ES_AUTOHSCROLL, 0, 0, 10, 10, g_hwnd, (HMENU)(INT_PTR)(100 + e), hi, NULL);
    app_init(&HOST, g_frameless, lang, px);
    if (selftest) app_enable_selftest(selftest);
    ShowWindow(g_hwnd, show); UpdateWindow(g_hwnd);
    MSG msg; int code = 0;
    while (GetMessageW(&msg, NULL, 0, 0) > 0) { TranslateMessage(&msg); DispatchMessageW(&msg); code = (int)msg.wParam; }
    return code;
}
