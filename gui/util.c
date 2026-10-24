#include "util.h"
#include <stdlib.h>
#include <string.h>

#define RING 512
static wchar_t *ring[RING];
static int ring_i;

const wchar_t *w8(const char *s) {
    if (!s) s = "";
    size_t n = strlen(s);
    wchar_t *o = (wchar_t *)malloc((n + 1) * sizeof(wchar_t) * 2);
    size_t k = 0;
    const unsigned char *p = (const unsigned char *)s;
    while (*p) {
        unsigned cp = *p++;
        int more = 0;
        if (cp >= 0xF0) { cp &= 7; more = 3; } else if (cp >= 0xE0) { cp &= 15; more = 2; } else if (cp >= 0xC0) { cp &= 31; more = 1; } else if (cp >= 0x80) { cp = 0xFFFD; }
        while (more-- > 0 && (*p & 0xC0) == 0x80) cp = (cp << 6) | (*p++ & 63);
        if (sizeof(wchar_t) == 2 && cp > 0xFFFF) { cp -= 0x10000; o[k++] = (wchar_t)(0xD800 + (cp >> 10)); o[k++] = (wchar_t)(0xDC00 + (cp & 0x3FF)); }
        else o[k++] = (wchar_t)cp;
    }
    o[k] = 0;
    free(ring[ring_i]);
    ring[ring_i] = o;
    ring_i = (ring_i + 1) % RING;
    return o;
}

char *u8w(const wchar_t *w) {
    size_t n = wcslen(w), k = 0;
    char *o = (char *)malloc(n * 4 + 1);
    for (size_t i = 0; i < n; i++) {
        unsigned cp = (unsigned)w[i];
        if (sizeof(wchar_t) == 2 && cp >= 0xD800 && cp < 0xDC00 && i + 1 < n) cp = 0x10000 + ((cp - 0xD800) << 10) + ((unsigned)w[++i] - 0xDC00);
        if (cp < 0x80) o[k++] = (char)cp;
        else if (cp < 0x800) { o[k++] = (char)(0xC0 | (cp >> 6)); o[k++] = (char)(0x80 | (cp & 63)); }
        else if (cp < 0x10000) { o[k++] = (char)(0xE0 | (cp >> 12)); o[k++] = (char)(0x80 | ((cp >> 6) & 63)); o[k++] = (char)(0x80 | (cp & 63)); }
        else { o[k++] = (char)(0xF0 | (cp >> 18)); o[k++] = (char)(0x80 | ((cp >> 12) & 63)); o[k++] = (char)(0x80 | ((cp >> 6) & 63)); o[k++] = (char)(0x80 | (cp & 63)); }
    }
    o[k] = 0;
    return o;
}

char *xstrdup(const char *s) { size_t n = strlen(s) + 1; char *p = (char *)malloc(n); memcpy(p, s, n); return p; }
