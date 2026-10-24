#include "i18n.h"
#include "util.h"
#include "third_party/cJSON.h"
#include <stdlib.h>
#include <string.h>

extern const char *const LANG_JSON[4];   /* generated: ko, zh-Hans, zh-Hant, en */
static const char *const TAGS[4] = {"ko", "zh-Hans", "zh-Hant", "en"};
static const char *const NATIVE[4] = {"\xED\x95\x9C\xEA\xB5\xAD\xEC\x96\xB4", "\xE7\xAE\x80\xE4\xBD\x93\xE4\xB8\xAD\xE6\x96\x87", "\xE7\xB9\x81\xE9\xAB\x94\xE4\xB8\xAD\xE6\x96\x87", "English"};
/* Tahoma first for Latin (Windows look); each CJK language uses the face that has its glyph forms */
static const char *const FACE[4] = {"Malgun Gothic", "Microsoft YaHei", "Microsoft JhengHei", "Tahoma"};
static cJSON *packs[4];
static int cur;

void i18n_init(void) { for (int i = 0; i < 4; i++) packs[i] = cJSON_Parse(LANG_JSON[i]); }
void i18n_set(const char *tag) { for (int i = 0; i < 4; i++) if (!strcmp(tag, TAGS[i])) cur = i; }
const char *i18n_tag(void) { return TAGS[cur]; }
int i18n_count(void) { return 4; }
const char *i18n_tag_at(int i) { return TAGS[i]; }
const char *i18n_native_name(int i) { return NATIVE[i]; }
const char *i18n_font_face(void) { return FACE[cur]; }

static const char *raw(const char *key) {
    cJSON *v = cJSON_GetObjectItemCaseSensitive(packs[cur], key);
    if (!cJSON_IsString(v)) v = cJSON_GetObjectItemCaseSensitive(packs[3], key);
    return cJSON_IsString(v) ? v->valuestring : key;
}

static char *strip_mnemonic(const char *key, const char *s) {
    char *o = xstrdup(s);
    if (strncmp(key, "menu_", 5) && strncmp(key, "btn_", 4) && strncmp(key, "chk_", 4)) return o;
    char *d = o;
    for (const char *p = s; *p; p++) if (*p != '_' || !p[1] || p[1] == ' ') *d++ = *p;
    *d = 0;
    return o;
}

const wchar_t *T(const char *key) {
    char *s = strip_mnemonic(key, raw(key));
    const wchar_t *w = w8(s);
    free(s);
    return w;
}

const wchar_t *TF(const char *key, int n, const char **a) {
    const char *f = raw(key);
    size_t cap = strlen(f) + 1;
    for (int i = 0; i < n; i++) cap += strlen(a[i]) * 4;
    char *o = (char *)malloc(cap + 8), *d = o;
    for (const char *p = f; *p; p++) {
        if (p[0] == '{' && p[1] >= '0' && p[1] <= '9' && p[2] == '}') { int i = p[1] - '0'; if (i < n) { strcpy(d, a[i]); d += strlen(a[i]); } p += 2; }
        else *d++ = *p;
    }
    *d = 0;
    const wchar_t *w = w8(o);
    free(o);
    return w;
}
