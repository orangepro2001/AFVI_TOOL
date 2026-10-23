package policy_test

import (
	"testing"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/policy"
)

var tr = i18n.For(i18n.En)

func ref(m string, k machine.NodeKind) policy.Ref { return policy.Ref{Machine: m, Node: k} }

func TestCheckCopy(t *testing.T) {
	const A, B = "AFVI 13", "AFVI 14"
	cases := []struct {
		name     string
		src, dst policy.Ref
		rename   bool
		ok       bool
	}{
		{"FM1 -> FM2 same machine", ref(A, "FM1"), ref(A, "FM2"), false, true},
		{"FM2 -> FM1 same machine", ref(A, "FM2"), ref(A, "FM1"), false, true},
		{"FM1 -> FM1 same name", ref(A, "FM1"), ref(A, "FM1"), false, false},
		{"FM1 -> FM1 new name", ref(A, "FM1"), ref(A, "FM1"), true, true},
		{"FM1 -> other machine FM1 (single)", ref(A, "FM1"), ref(B, "FM1"), false, false},
		{"FM1 -> other machine FM2 (single)", ref(A, "FM1"), ref(B, "FM2"), true, false},
		{"FM1 -> BM", ref(A, "FM1"), ref(A, "BM"), false, false},
		{"FM2 -> BM other machine", ref(A, "FM2"), ref(B, "BM"), true, false},
		{"BM -> FM1", ref(A, "BM"), ref(A, "FM1"), false, false},
		{"BM -> FM other machine", ref(A, "BM"), ref(B, "FM1"), false, false},
		{"BM -> own BM same name", ref(A, "BM"), ref(A, "BM"), false, false},
		{"BM -> own BM new name", ref(A, "BM"), ref(A, "BM"), true, false},
		{"BM -> other BM", ref(A, "BM"), ref(B, "BM"), false, true},
		{"BM -> other BM new name", ref(A, "BM"), ref(B, "BM"), true, true},
		{"case-insensitive machine names", ref(A, "FM1"), ref("afvi 13", "FM2"), false, true},
		{"unknown node", ref(A, "XX"), ref(A, "FM1"), false, false},
	}
	for _, c := range cases {
		err := policy.CheckCopy(tr, c.src, c.dst, c.rename)
		if (err == nil) != c.ok {
			t.Errorf("%s: err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestGroupAndTargets(t *testing.T) {
	if policy.CheckGroupCopy(tr, "AFVI 13", "afvi 13") == nil {
		t.Error("group copy onto itself must be refused")
	}
	if err := policy.CheckGroupCopy(tr, "AFVI 13", "AFVI 14"); err != nil {
		t.Error(err)
	}
	got := policy.Targets(ref("AFVI 13", "BM"), []string{"AFVI 13", "AFVI 14"})
	if len(got) != 1 || got[0] != ref("AFVI 14", "BM") {
		t.Errorf("BM targets: %+v", got)
	}
	got = policy.Targets(ref("AFVI 13", "FM1"), []string{"AFVI 13", "AFVI 14"})
	if len(got) != 2 { // FM1 (copy+rename) and FM2 of the same machine
		t.Errorf("FM1 targets: %+v", got)
	}
}
