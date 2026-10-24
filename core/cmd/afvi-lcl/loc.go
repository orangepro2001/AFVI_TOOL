package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed lang/*.json
var langFS embed.FS

// langTags lists the packs in menu order; the first is the default.
var langTags = []string{"ko", "zh-Hans", "zh-Hant", "en"}

const defaultLang = "ko"

// Loc holds the four language packs (flat key -> text, same files as the GUI packs of the Avalonia app).
type Loc struct {
	packs map[string]map[string]string
	tag   string
}

func newLoc() *Loc {
	l := &Loc{packs: map[string]map[string]string{}, tag: defaultLang}
	for _, t := range langTags {
		b, err := langFS.ReadFile("lang/" + t + ".json")
		if err != nil {
			panic(err)
		}
		m := map[string]string{}
		if err := json.Unmarshal(b, &m); err != nil {
			panic(err)
		}
		// the packs mark the mnemonic with "_"; the LCL uses "&"
		for k, v := range m {
			if strings.HasPrefix(k, "menu_") || strings.HasPrefix(k, "btn_") {
				m[k] = strings.Replace(v, "_", "&", 1)
			}
		}
		l.packs[t] = m
	}
	return l
}

func (l *Loc) valid(tag string) bool { _, ok := l.packs[tag]; return ok }

func (l *Loc) Set(tag string) {
	if l.valid(tag) {
		l.tag = tag
	}
}

func (l *Loc) Tag() string { return l.tag }

// Name is the native name of a language (shown in the language menu).
func (l *Loc) Name(tag string) string { return l.packs[tag]["_name"] }

// T returns the text of key with {0}, {1} ... replaced by args.
func (l *Loc) T(key string, args ...any) string {
	s, ok := l.packs[l.tag][key]
	if !ok {
		if s, ok = l.packs["en"][key]; !ok {
			return key
		}
	}
	for i, a := range args {
		s = strings.ReplaceAll(s, fmt.Sprintf("{%d}", i), fmt.Sprint(a))
	}
	return s
}

// The saved language lives in the same ui.json the Avalonia app uses.
func uiSettingsPath() string {
	root := os.Getenv("LOCALAPPDATA")
	if root == "" {
		if x := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(x) {
			root = x
		} else if h, err := os.UserHomeDir(); err == nil {
			root = filepath.Join(h, ".local", "share")
		}
	}
	if root == "" || !filepath.IsAbs(root) {
		root = os.TempDir()
	}
	return filepath.Join(root, "AFVI_TOOLS", "ui.json")
}

func loadLanguage() string {
	b, err := os.ReadFile(uiSettingsPath())
	if err != nil {
		return defaultLang
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return defaultLang
	}
	if s, ok := m["language"].(string); ok && s != "" {
		return s
	}
	return defaultLang
}

func saveLanguage(tag string) {
	p := uiSettingsPath()
	m := map[string]any{}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	m["language"] = tag
	b, _ := json.Marshal(m)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, b, 0o644) // a preference, never fatal
}
