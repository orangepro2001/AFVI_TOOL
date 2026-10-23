// Package i18n holds the language packs of the core: every sentence the
// user can read (plan steps, warnings, refusals, job log) is a key here, in
// Korean (the default), Simplified and Traditional Chinese and English.
//
// The GUI sends its language in the X-Afvi-Lang header; the server puts it in
// the request context and the planners translate with it. Low level technical
// errors (OS errors, mismatch details) stay English on purpose.
package i18n

import (
	"context"
	"fmt"
	"strings"
)

// Lang is a language tag of the packs.
type Lang string

const (
	Ko     Lang = "ko"
	ZhHans Lang = "zh-Hans"
	ZhHant Lang = "zh-Hant"
	En     Lang = "en"
	// Default is the language the program starts in.
	Default = Ko
)

// All lists the packs in menu order.
var All = []Lang{Ko, ZhHans, ZhHant, En}

// Parse maps a tag such as "ko-KR", "zh-CN", "zh-TW", "zh-Hant-HK" or "en-US"
// to a pack; unknown or empty input gives Default.
func Parse(s string) Lang {
	s = strings.ToLower(strings.TrimSpace(s))
	switch {
	case s == "":
		return Default
	case strings.HasPrefix(s, "ko"):
		return Ko
	case strings.HasPrefix(s, "en"):
		return En
	case strings.HasPrefix(s, "zh"):
		if strings.Contains(s, "hant") || strings.HasSuffix(s, "-tw") || strings.HasSuffix(s, "-hk") || strings.HasSuffix(s, "-mo") {
			return ZhHant
		}
		return ZhHans
	}
	return Default
}

type ctxKey struct{}

// With returns a context carrying the language.
func With(ctx context.Context, l Lang) context.Context { return context.WithValue(ctx, ctxKey{}, l) }

// From returns the language of the context, Default when none is set.
func From(ctx context.Context) Lang {
	if l, ok := ctx.Value(ctxKey{}).(Lang); ok {
		return l
	}
	return Default
}

// T translates a key with fmt arguments.
type T func(key string, args ...any) string

// For returns the translator of a language. A missing key falls back to
// English and then to the key itself, so a gap in a pack is visible but
// never fatal.
func For(l Lang) T {
	pack := catalog[l]
	return func(key string, args ...any) string {
		s, ok := pack[key]
		if !ok {
			if s, ok = catalog[En][key]; !ok {
				return key
			}
		}
		if len(args) == 0 {
			return s
		}
		return fmt.Sprintf(s, args...)
	}
}

// FromContext is For(From(ctx)).
func FromContext(ctx context.Context) T { return For(From(ctx)) }

// Keys returns every key of the English pack (used by tests).
func Keys() []string {
	out := make([]string, 0, len(catalog[En]))
	for k := range catalog[En] {
		out = append(out, k)
	}
	return out
}

type row struct{ key, ko, hans, hant, en string }

var catalog = build(rows)

func build(rs []row) map[Lang]map[string]string {
	m := map[Lang]map[string]string{Ko: {}, ZhHans: {}, ZhHant: {}, En: {}}
	for _, r := range rs {
		m[Ko][r.key], m[ZhHans][r.key], m[ZhHant][r.key], m[En][r.key] = r.ko, r.hans, r.hant, r.en
	}
	return m
}
