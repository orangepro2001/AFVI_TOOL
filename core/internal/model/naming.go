// Package model knows how a "model" (型号) is laid out on one machine node.
//
// A node (FM1, FM2 or BM) exposes two shares and a model called N (case does
// not matter anywhere) is the union of
//
//	PxRepository/N                 directory, the teach data (<res>/Line/...)
//	PxRepository/N.his             optional history file, first CSV column is N
//	PxInventory/INSPECT_SPEC/N-NN  directories, NN = two digits (normally -00)
//	PxInventory/LIGHT_SPEC/N       directory
//
// Observed on 192.168.1.61/62/63 on 2026-10-24; see docs/NAMING.md.
package model

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"afvi/tools/core/internal/i18n"
)

// Key is the case-insensitive identity of a name.
func Key(name string) string { return strings.ToLower(name) }

// Equal reports whether two names are the same model name.
func Equal(a, b string) bool { return Key(a) == Key(b) }

// Naming classes seen in the field. They are a hint for the list, not a rule.
const (
	ClassStandard = "A"     // 21csp043-09   NN aaa NNN-NN
	ClassStrip    = "B"     // 2AN0859F01    N aa NNNN a NN
	ClassBase     = "C"     // 6sg0504a      N aa NNNN a
	ClassM99      = "M99"   // 7M99-535, M99-460
	ClassCopy     = "COPY"  // bk, TEST, 复制本, 原本
	ClassOther    = "OTHER" // danny, Sample ...
)

var (
	reA   = regexp.MustCompile(`(?i)^\d{2}[a-z]{3}\d{3}-\d{2}$`)
	reB   = regexp.MustCompile(`(?i)^\d[a-z]{2}\d{4}[a-z]\d{2}$`)
	reC   = regexp.MustCompile(`(?i)^\d[a-z]{2}\d{4}[a-z]$`)
	reM99 = regexp.MustCompile(`(?i)^(\d)?m99-\d{3}(-\d+)?$`)
	reCpy = regexp.MustCompile(`(?i)(bk|test|copy|복사본|원본|backup)`)
)

// Classify returns the naming class of a model name.
func Classify(name string) string {
	switch {
	case reA.MatchString(name):
		return ClassStandard
	case reB.MatchString(name):
		return ClassStrip
	case reC.MatchString(name):
		return ClassBase
	case reM99.MatchString(name):
		return ClassM99
	case reCpy.MatchString(name):
		return ClassCopy
	}
	return ClassOther
}

const MaxNameRunes = 100

var reserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidateName checks a name the user typed for a new model. The names must
// be legal on Windows because the machines run Windows, even when this tool
// runs on Linux.
func ValidateName(tr i18n.T, name string) error {
	if name == "" {
		return errors.New(tr("n.empty"))
	}
	if name != strings.TrimSpace(name) {
		return errors.New(tr("n.space"))
	}
	if utf8.RuneCountInString(name) > MaxNameRunes {
		return errors.New(tr("n.too_long", MaxNameRunes))
	}
	if !utf8.ValidString(name) {
		return errors.New(tr("n.utf8"))
	}
	for _, r := range name {
		if r < 0x20 || strings.ContainsRune(`<>:"/\|?*`, r) || unicode.IsControl(r) {
			return errors.New(tr("n.bad_char", r))
		}
	}
	if strings.HasSuffix(name, ".") {
		return errors.New(tr("n.dot_end"))
	}
	base := name
	if i := strings.IndexByte(base, '.'); i >= 0 {
		base = base[:i]
	}
	if reserved[Key(base)] {
		return errors.New(tr("n.reserved", base))
	}
	if strings.HasSuffix(Key(name), ".his") {
		return errors.New(tr("n.his_end"))
	}
	return nil
}
