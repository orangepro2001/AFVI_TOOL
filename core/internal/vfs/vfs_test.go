package vfs_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"afvi/tools/core/internal/vfs"
)

func TestClean(t *testing.T) {
	cases := map[string]string{"": "", ".": "", "/": "", "a/b": "a/b", `a\b`: "a/b", "../../x": "x", "a/../..": "", "/a//b/": "a/b"}
	for in, want := range cases {
		if got := vfs.Clean(in); got != want {
			t.Errorf("Clean(%q)=%q want %q", in, got, want)
		}
	}
}

func TestReadOnlyOSFSRefusesEveryWrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644)
	f := vfs.NewOS(dir, true)
	if _, err := f.Create("g"); !errors.Is(err, vfs.ErrReadOnly) {
		t.Error("Create")
	}
	for name, err := range map[string]error{
		"MkdirAll": f.MkdirAll("d"), "Remove": f.Remove("f"), "RemoveAll": f.RemoveAll("f"),
		"Rename": f.Rename("f", "g"), "Chtimes": f.Chtimes("f", time.Now()),
	} {
		if !errors.Is(err, vfs.ErrReadOnly) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "f")); err != nil {
		t.Error("file vanished")
	}
	if _, err := f.ReadDir(""); err != nil {
		t.Error("reading must still work")
	}
}

func TestPathEscapeIsContained(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "outside-"+filepath.Base(dir))
	os.WriteFile(outside, []byte("secret"), 0o644)
	defer os.Remove(outside)
	f := vfs.NewOS(dir, false)
	if _, err := f.Stat("../" + filepath.Base(outside)); err == nil {
		t.Error("escaped the root")
	}
	if err := f.RemoveAll(""); err == nil {
		t.Error("removing the share root must be refused")
	}
}

func TestUNCParsing(t *testing.T) {
	h, s, sub, err := vfs.SplitUNC(`\\192.168.1.61\PxRepository\a\b`)
	if err != nil || h != "192.168.1.61" || s != "PxRepository" || sub != "a/b" {
		t.Errorf("%q %q %q %v", h, s, sub, err)
	}
	if _, _, _, err := vfs.SplitUNC(`\\host`); err == nil {
		t.Error("share is required")
	}
	if !vfs.IsUNC("//h/s") || vfs.IsUNC("/tmp/x") {
		t.Error("IsUNC")
	}
}

func TestOpenUNCOnLinuxIsReadOnlySMB(t *testing.T) {
	f, err := vfs.Open(`\\192.0.2.1\PxRepository`, vfs.Options{AllowNetworkWrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if hasWindows() {
		return
	}
	if !f.ReadOnly() {
		t.Error("SMB access from Linux must stay read-only even when the lock is lifted")
	}
}
