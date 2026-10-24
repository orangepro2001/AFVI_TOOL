package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var placeholder = regexp.MustCompile(`\{\d+\}`)

func TestPacksAreCompleteWithSamePlaceholders(t *testing.T) {
	l := newLoc()
	en := l.packs["en"]
	if len(en) < 100 {
		t.Fatalf("English pack has only %d keys", len(en))
	}
	for _, tag := range langTags {
		for k, v := range en {
			got, ok := l.packs[tag][k]
			if !ok || got == "" {
				t.Errorf("%s lacks %q", tag, k)
				continue
			}
			if a, b := strings.Join(placeholder.FindAllString(v, -1), ","), strings.Join(placeholder.FindAllString(got, -1), ","); a != b {
				t.Errorf("%s/%s: placeholders [%s] vs English [%s]", tag, k, b, a)
			}
		}
	}
}

func TestDefaultLanguageIsKorean(t *testing.T) {
	l := newLoc()
	if defaultLang != "ko" || l.Tag() != "ko" || l.T("tab_copy") != "모델 복사" {
		t.Fatalf("default language: %q %q", l.Tag(), l.T("tab_copy"))
	}
	l.Set("zh-Hant")
	if l.T("lbl_source_is", "A / FM1") != "來源: A / FM1" {
		t.Fatalf("zh-Hant: %q", l.T("lbl_source_is", "A / FM1"))
	}
	l.Set("xx") // unknown: unchanged
	if l.Tag() != "zh-Hant" {
		t.Fatal("unknown tag changed the language")
	}
	if l.T("no.such.key") != "no.such.key" {
		t.Fatal("missing key must show the key")
	}
}

func TestMnemonicsUseLCLAmpersand(t *testing.T) {
	l := newLoc()
	if got := l.packs["en"]["menu_file"]; got != "&File" {
		t.Fatalf("menu_file = %q", got)
	}
}

// every key the program asks for exists in the packs
func TestEveryKeyUsedInSourceExists(t *testing.T) {
	re := regexp.MustCompile(`\.T\("([a-z0-9_]+)"[,)]`)
	l := newLoc()
	files, _ := filepath.Glob("*.go")
	n := 0
	for _, f := range files {
		b, _ := os.ReadFile(f)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			n++
			if _, ok := l.packs["en"][m[1]]; !ok {
				t.Errorf("%s uses unknown key %q", f, m[1])
			}
		}
	}
	if n < 40 {
		t.Fatalf("found only %d key uses", n)
	}
}

func TestSkinsDifferAndDefaultsFollowThePlatform(t *testing.T) {
	pickSkin("win2000")
	a := curSkin.Face()
	pickSkin("platinum")
	b := curSkin.Face()
	pickSkin("native")
	if a == b || curSkin != nil {
		t.Fatalf("skins: win2000 %x platinum %x native %v", a, b, curSkin)
	}
	if rgb(0xD4, 0xD0, 0xC8) != 0xC8D0D4 {
		t.Fatalf("TColor must be BGR: %x", rgb(0xD4, 0xD0, 0xC8))
	}
}
