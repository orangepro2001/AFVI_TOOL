package i18n_test

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"afvi/tools/core/internal/i18n"
)

func TestParse(t *testing.T) {
	cases := map[string]i18n.Lang{
		"": i18n.Ko, "ko": i18n.Ko, "ko-KR": i18n.Ko, "KO": i18n.Ko,
		"en": i18n.En, "en-US": i18n.En,
		"zh": i18n.ZhHans, "zh-CN": i18n.ZhHans, "zh-Hans": i18n.ZhHans, "zh-SG": i18n.ZhHans,
		"zh-TW": i18n.ZhHant, "zh-Hant": i18n.ZhHant, "zh-HK": i18n.ZhHant, "zh-Hant-TW": i18n.ZhHant,
		"fr": i18n.Ko, "xx": i18n.Ko,
	}
	for in, want := range cases {
		if got := i18n.Parse(in); got != want {
			t.Errorf("Parse(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestDefaultIsKorean(t *testing.T) {
	if i18n.Default != i18n.Ko || i18n.From(context.Background()) != i18n.Ko {
		t.Fatal("the program must start in Korean")
	}
	if got := i18n.For(i18n.Default)("o.same_name"); got != "새 이름이 현재 이름과 같습니다" {
		t.Errorf("default text: %q", got)
	}
}

var verb = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]`)

// Every pack has every key, with the same format verbs in the same order.
func TestCatalogComplete(t *testing.T) {
	keys := i18n.Keys()
	if len(keys) < 60 {
		t.Fatalf("suspiciously few keys: %d", len(keys))
	}
	for _, k := range keys {
		want := strings.Join(verb.FindAllString(i18n.For(i18n.En)(k), -1), " ")
		for _, l := range i18n.All {
			// call without args returns the raw format string
			s := i18n.For(l)(k)
			if s == "" || s == k {
				t.Errorf("%s: key %q missing", l, k)
				continue
			}
			if got := strings.Join(verb.FindAllString(s, -1), " "); got != want {
				t.Errorf("%s: key %q has verbs [%s], English has [%s]", l, k, got, want)
			}
		}
	}
}

func TestFallback(t *testing.T) {
	if got := i18n.For(i18n.ZhHant)("no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key: %q", got)
	}
	if got := i18n.For(i18n.En)("m.dup", "X"); got != `Duplicate machine name: "X"` {
		t.Errorf("format: %q", got)
	}
}
