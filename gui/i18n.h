#ifndef I18N_H
#define I18N_H
#include <wchar.h>
/* The four packs are the same flat JSON files as the other front ends (embedded by gen_lang.py). */
void i18n_init(void);
void i18n_set(const char *tag);          /* "ko" "zh-Hans" "zh-Hant" "en" */
const char *i18n_tag(void);
int i18n_count(void);
const char *i18n_tag_at(int i);
const char *i18n_native_name(int i);
const wchar_t *T(const char *key);                          /* text of a key, access-key underscore stripped */
const wchar_t *TF(const char *key, int n, const char **a);  /* with {0}..{n-1} replaced by UTF-8 args */
const char *i18n_font_face(void);        /* GDI face for the current language */
#endif
