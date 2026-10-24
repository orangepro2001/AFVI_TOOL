#ifndef UTIL_H
#define UTIL_H
#include <stddef.h>
#include <wchar.h>
/* UTF-8 -> wchar_t (UTF-16 on Windows, UTF-32 elsewhere). The result lives in a ring of 512 buffers: use it
 * immediately, never keep it. */
const wchar_t *w8(const char *utf8);
/* wchar_t -> UTF-8 into a malloc'ed string (caller frees) */
char *u8w(const wchar_t *w);
char *xstrdup(const char *s);
#endif
