// MSVC-ABI symbols the NativeAOT runtime libraries expect from the MSVC C++ runtime (libcmt/libcpmt), provided here with MSVC mangled names.
#include <stdlib.h>
#include <stddef.h>
void *new_(size_t n)                __asm__("??2@YAPEAX_K@Z");
void *new_nt(size_t n, const void*) __asm__("??2@YAPEAX_KAEBUnothrow_t@std@@@Z");
void *newa_nt(size_t n, const void*)__asm__("??_U@YAPEAX_KAEBUnothrow_t@std@@@Z");
void  del(void *p)                  __asm__("??3@YAXPEAX@Z");
void  dels(void *p, size_t n)       __asm__("??3@YAXPEAX_K@Z");
void  dela(void *p)                 __asm__("??_V@YAXPEAX@Z");
void *new_(size_t n) { void *p = malloc(n ? n : 1); if (!p) abort(); return p; }
void *new_nt(size_t n, const void *t) { (void)t; return malloc(n ? n : 1); }
void *newa_nt(size_t n, const void *t) { (void)t; return malloc(n ? n : 1); }
void del(void *p) { free(p); }
void dels(void *p, size_t n) { (void)n; free(p); }
void dela(void *p) { free(p); }
char nothrow_obj __asm__("?nothrow@std@@3Unothrow_t@1@B") = 0;
int _fltused = 0;
void _purecall(void) { abort(); }

// ---- MSVC CRT startup / security pieces (no libcmt here) ----
#include <stdint.h>
uintptr_t __security_cookie = 0x2B992DDFA232ULL;
void __security_check_cookie(uintptr_t c) { if (c != __security_cookie) abort(); }
void __report_rangecheckfailure(void) { abort(); }
int __GSHandlerCheck(void *a, void *b, void *c, void *d) { (void)a; (void)b; (void)c; (void)d; return 1; }
void __guard_dispatch_icall_dummy(void) {}
char __tls_guard = 0;
void __dyn_tls_on_demand_init(void) {}

extern int wmain(int, unsigned short **);
extern void __wgetmainargs(int *, unsigned short ***, unsigned short ***, int, void *);
__declspec(dllimport) void __stdcall ExitProcess(unsigned int);
int wmainCRTStartup(void) {
  int argc = 0, si = 0; unsigned short **argv = 0, **envp = 0;
  __wgetmainargs(&argc, &argv, &envp, 0, &si);
  ExitProcess((unsigned)wmain(argc, argv));
  return 0;
}
int __stdcall WinMain(void *a, void *b, char *c, int d) { (void)a; (void)b; (void)c; (void)d; return 0; }
