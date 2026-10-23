package model_test

import (
	"strings"
	"testing"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/model"
	"afvi/tools/core/internal/testkit"
)

var tr = i18n.For(i18n.En)

func TestClassify(t *testing.T) {
	cases := map[string]string{
		"21csp043-09": model.ClassStandard, "24FBO060-19": model.ClassStandard,
		"2AN0859F01": model.ClassStrip, "6AN0921I01": model.ClassStrip,
		"6sg0504a": model.ClassBase, "3hn1008o": model.ClassBase,
		"7M99-535": model.ClassM99, "M99-460": model.ClassM99,
		"7M99-586-TEST": model.ClassCopy, "7M99-631bk": model.ClassCopy, "6AN0921I01 - 복사본 (2)": model.ClassCopy,
		"danny": model.ClassOther, "Sample": model.ClassOther,
	}
	for in, want := range cases {
		if got := model.Classify(in); got != want {
			t.Errorf("Classify(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestValidateName(t *testing.T) {
	good := []string{"21csp043-09", "7M99-535", "M99-493 gerber new", "6AN0921I01 - 복사본 (2)", "a.b"}
	for _, n := range good {
		if err := model.ValidateName(tr, n); err != nil {
			t.Errorf("ValidateName(%q) = %v, want ok", n, err)
		}
	}
	bad := []string{"", " x", "x ", "a/b", `a\b`, "a:b", "a*b", "a?b", `a"b`, "a<b", "x.", "CON", "nul.txt", "x.his", "X.HIS", strings.Repeat("x", 101), "a\x01b"}
	for _, n := range bad {
		if err := model.ValidateName(tr, n); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", n)
		}
	}
}

func TestRewriteHistory(t *testing.T) {
	in := "24fbo060-19,0,1\r\n24FBO060-19,0,2\r\nother,0,3\r\n24fbo060-19-x,0,4\nlast-no-newline"
	out, n := model.RewriteHistory([]byte(in), "24fbo060-19", "NEW")
	want := "NEW,0,1\r\nNEW,0,2\r\nother,0,3\r\n24fbo060-19-x,0,4\nlast-no-newline"
	if string(out) != want || n != 2 {
		t.Errorf("got %q (%d), want %q (2)", out, n, want)
	}
	out, n = model.RewriteHistory([]byte("x,1"), "y", "z")
	if string(out) != "x,1" || n != 0 {
		t.Errorf("untouched case: %q %d", out, n)
	}
}

func TestScanCaseInsensitiveBundle(t *testing.T) {
	s := testkit.NewStore(t, t.TempDir())
	// the real field pattern: directory lower case, .his upper case, specs mixed
	testkit.Write(t, s.Repo, "24fbo060-19/x/y.txt", "1")
	testkit.Write(t, s.Repo, "24FBO060-19.his", "24fbo060-19,0\r\n")
	testkit.Write(t, s.Inv, "INSPECT_SPEC/24FBO060-19-00/a.xml", "1")
	testkit.Write(t, s.Inv, "INSPECT_SPEC/24FBO060-19-01/a.xml", "1")
	testkit.Write(t, s.Inv, "INSPECT_SPEC/24FBO060-19-00 스캔원본/a.xml", "1") // not part of the bundle
	testkit.Write(t, s.Inv, "INSPECT_SPEC/stray/a.xml", "1")               // unmanaged
	testkit.Write(t, s.Inv, "LIGHT_SPEC/24FBO060-19/l.xml", "1")
	testkit.Write(t, s.Repo, "danny/a.txt", "1") // repository only

	all, err := model.Scan(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 models, got %d: %+v", len(all), all)
	}
	var b model.Bundle
	for _, x := range all {
		if model.Equal(x.Name, "24fbo060-19") {
			b = x
		}
	}
	if b.Name != "24fbo060-19" {
		t.Errorf("canonical name should be the repository directory, got %q", b.Name)
	}
	if !b.Has(model.KindRepo) || !b.Has(model.KindHistory) || !b.Has(model.KindLight) || b.Count(model.KindInspect) != 2 {
		t.Errorf("bundle incomplete: %+v", b.Parts)
	}
	for _, p := range b.Parts {
		if !p.Belongs("24Fbo060-19") {
			t.Errorf("%+v should belong to the model", p)
		}
	}
	got, found, err := model.Locate(s, "24FBO060-19")
	if err != nil || !found || len(got.Parts) != len(b.Parts) {
		t.Errorf("Locate: %v %v %+v", err, found, got)
	}
	if _, found, _ := model.Locate(s, "nope"); found {
		t.Error("Locate found a missing model")
	}
}

func TestBelongsRejectsOthers(t *testing.T) {
	p := model.Part{Kind: model.KindRepo, Share: model.ShareRepo, Name: "abc"}
	if p.Belongs("abcd") || p.Belongs("ab") {
		t.Error("Belongs must compare the full name")
	}
	q := model.Part{Kind: model.KindInspect, Share: model.ShareInv, Dir: "INSPECT_SPEC", Name: "abc-00", Suffix: "-00"}
	if !q.Belongs("ABC") || q.Belongs("abc-00") {
		t.Error("inspect belongs by base name")
	}
}
