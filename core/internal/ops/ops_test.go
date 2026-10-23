package ops_test

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/model"
	"afvi/tools/core/internal/ops"
	"afvi/tools/core/internal/policy"
	"afvi/tools/core/internal/testkit"
	"afvi/tools/core/internal/vfs"
)

// enCtx asks for English so the assertions on message text are stable.
func enCtx() context.Context { return i18n.With(context.Background(), i18n.En) }

func ref(m string, k machine.NodeKind) policy.Ref { return policy.Ref{Machine: m, Node: k} }

// world builds machines A and B with six nodes below a temp dir.
func world(t *testing.T) (testkit.Env, string) {
	t.Helper()
	root := t.TempDir()
	env := testkit.Env{}
	for _, m := range []string{"A", "B"} {
		for _, k := range machine.Kinds {
			env[ref(m, k)] = testkit.NewStore(t, filepath.Join(root, m, string(k)))
		}
	}
	return env, root
}

func run(t *testing.T, pl *ops.Plan) error {
	t.Helper()
	return ops.Run(context.Background(), pl, ops.NewManager().ProgressForTest())
}

func TestCopyReplacesTargetCaseInsensitively(t *testing.T) {
	env, _ := world(t)
	src, dst := env[ref("A", "FM1")], env[ref("A", "FM2")]
	testkit.AddModel(t, src, "24fbo060-19", "NEW")
	// the target has the same model in other case plus a file only the old one has
	testkit.AddModel(t, dst, "24FBO060-19", "OLD")
	testkit.Write(t, dst.Repo, "24FBO060-19/only-in-old.txt", "x")
	testkit.Write(t, dst.Inv, "INSPECT_SPEC/24FBO060-19-01/old.xml", "x") // extra variant must go too
	testkit.Write(t, dst.Repo, "unrelated/keep.txt", "keep")

	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{
		Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "24FBO060-19"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Blockers) != 0 || len(pl.Warnings) == 0 {
		t.Fatalf("blockers=%v warnings=%v", pl.Blockers, pl.Warnings)
	}
	if err := run(t, pl); err != nil {
		t.Fatal(err)
	}
	if got := testkit.Read(t, dst.Repo, "24fbo060-19/2.0/Line/Top/Top_handler.txt"); got != "handler NEW" {
		t.Errorf("repository content: %q", got)
	}
	if testkit.Exists(dst.Repo, "24FBO060-19") {
		t.Error("old directory name should have been replaced by the source's name")
	}
	if testkit.Exists(dst.Repo, "24fbo060-19/only-in-old.txt") || testkit.Exists(dst.Inv, "INSPECT_SPEC/24FBO060-19-01") {
		t.Error("stale target content survived")
	}
	if got := testkit.Read(t, dst.Inv, "LIGHT_SPEC/24fbo060-19/LightSpec.xml"); got != "light NEW" {
		t.Errorf("light spec: %q", got)
	}
	if got := testkit.Read(t, dst.Repo, "unrelated/keep.txt"); got != "keep" {
		t.Error("unrelated model was touched")
	}
	// the source is untouched
	if got := testkit.Read(t, src.Repo, "24fbo060-19/2.0/Line/Top/Top_handler.txt"); got != "handler NEW" {
		t.Error("source changed")
	}
}

func TestCopyAndRenameRewritesHistory(t *testing.T) {
	env, _ := world(t)
	src, dst := env[ref("A", "FM1")], env[ref("A", "FM2")]
	testkit.AddModel(t, src, "21csp043-09", "S")

	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{
		Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "21CSP043-09", NewName: "21csp043-10"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(t, pl); err != nil {
		t.Fatal(err)
	}
	his := testkit.Read(t, dst.Repo, "21csp043-10.his")
	if !strings.HasPrefix(his, "21csp043-10,0,0,1,Admin,20260507114036\r\n21csp043-10,0,0,2") {
		t.Errorf("history not rewritten: %q", his)
	}
	if !testkit.Exists(dst.Inv, "INSPECT_SPEC/21csp043-10-00/TOP/LIGHT0/InspectionSpec.xml") {
		t.Error("inspect variant suffix lost")
	}
	// source history is untouched
	if !strings.HasPrefix(testkit.Read(t, src.Repo, "21csp043-09.his"), "21csp043-09,") {
		t.Error("source history changed")
	}
}

func TestCopyRefusals(t *testing.T) {
	env, _ := world(t)
	testkit.AddModel(t, env[ref("A", "FM1")], "m1", "S")
	testkit.AddModel(t, env[ref("A", "BM")], "m1", "S")
	ctx := enCtx()
	bad := []ops.CopyRequest{
		{Source: ref("A", "FM1"), Target: ref("A", "BM"), Model: "m1"},                       // FM -> BM
		{Source: ref("A", "BM"), Target: ref("A", "FM1"), Model: "m1"},                       // BM -> FM
		{Source: ref("A", "BM"), Target: ref("A", "BM"), Model: "m1", NewName: "m2"},         // BM onto itself
		{Source: ref("A", "FM1"), Target: ref("B", "FM1"), Model: "m1"},                      // single node cross machine
		{Source: ref("A", "FM1"), Target: ref("A", "FM1"), Model: "m1"},                      // onto itself, same name
		{Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "missing"},                 // unknown model
		{Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "m1", NewName: "bad/name"}, // illegal name
		{Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: ""},
	}
	for i, r := range bad {
		if _, err := ops.PlanCopy(ctx, env, r); err == nil {
			t.Errorf("case %d: expected refusal for %+v", i, r)
		}
	}
	if _, err := ops.PlanCopy(ctx, env, ops.CopyRequest{Mode: ops.ModeGroup, Source: ref("A", "FM1"), Target: ref("A", "FM1"), Model: "m1"}); err == nil {
		t.Error("group copy onto the same machine must be refused")
	}
}

func TestCopyOnSameNodeWithNewName(t *testing.T) {
	env, _ := world(t)
	s := env[ref("A", "FM1")]
	testkit.AddModel(t, s, "m1", "S")
	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{
		Source: ref("A", "FM1"), Target: ref("A", "FM1"), Model: "m1", NewName: "m1-copy"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(t, pl); err != nil {
		t.Fatal(err)
	}
	if !testkit.Exists(s.Repo, "m1-copy/2.0/Line/Top/Top_handler.txt") || !testkit.Exists(s.Repo, "m1/2.0/Line/Top/Top_handler.txt") {
		t.Error("both models must exist")
	}
}

func TestGroupCopy(t *testing.T) {
	env, _ := world(t)
	testkit.AddModel(t, env[ref("A", "FM1")], "g1", "A-FM1")
	testkit.AddModel(t, env[ref("A", "FM2")], "g1", "A-FM2")
	// A's BM has no such model -> skipped with a warning; B's BM has an old one that must stay
	testkit.AddModel(t, env[ref("B", "BM")], "g1", "B-BM-OLD")
	testkit.AddModel(t, env[ref("B", "FM1")], "G1", "B-FM1-OLD")

	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{
		Mode: ops.ModeGroup, Source: ref("A", "FM1"), Target: ref("B", "FM1"), Model: "g1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Units) != 2 {
		t.Fatalf("want 2 units (FM1, FM2), got %d", len(pl.Units))
	}
	if !strings.Contains(strings.Join(pl.Warnings, "\n"), "A BM") {
		t.Errorf("missing skip warning: %v", pl.Warnings)
	}
	if err := run(t, pl); err != nil {
		t.Fatal(err)
	}
	if got := testkit.Read(t, env[ref("B", "FM1")].Repo, "g1/2.0/Line/Top/Top_handler.txt"); got != "handler A-FM1" {
		t.Errorf("B FM1: %q", got)
	}
	if got := testkit.Read(t, env[ref("B", "FM2")].Repo, "g1/2.0/Line/Top/Top_handler.txt"); got != "handler A-FM2" {
		t.Errorf("B FM2: %q", got)
	}
	if got := testkit.Read(t, env[ref("B", "BM")].Repo, "g1/2.0/Line/Top/Top_handler.txt"); got != "handler B-BM-OLD" {
		t.Errorf("B BM must stay untouched: %q", got)
	}
}

func TestBMCopyToOtherBM(t *testing.T) {
	env, _ := world(t)
	testkit.AddModel(t, env[ref("A", "BM")], "b1", "S")
	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{
		Source: ref("A", "BM"), Target: ref("B", "BM"), Model: "b1", NewName: "b2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(t, pl); err != nil {
		t.Fatal(err)
	}
	if !testkit.Exists(env[ref("B", "BM")].Repo, "b2/2.0/Line/Top/Top_handler.txt") {
		t.Error("b2 missing")
	}
}

func TestReadOnlyTargetIsBlocked(t *testing.T) {
	env, _ := world(t)
	testkit.AddModel(t, env[ref("A", "FM1")], "m1", "S")
	dst := env[ref("A", "FM2")]
	dst.Repo = vfs.NewOS(dst.Repo.Describe(), true)
	env[ref("A", "FM2")] = dst
	testkit.AddModel(t, model.Store{Repo: vfs.NewOS(dst.Repo.Describe(), false), Inv: dst.Inv}, "m1", "OLD")

	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Blockers) == 0 {
		t.Fatal("read-only target must block the plan")
	}
	if err := run(t, pl); !errors.Is(err, ops.ErrBlocked) {
		t.Fatalf("Run must refuse a blocked plan, got %v", err)
	}
	if got := testkit.Read(t, dst.Repo, "m1/2.0/Line/Top/Top_handler.txt"); got != "handler OLD" {
		t.Errorf("target changed: %q", got)
	}
}

// failFS makes Create fail for paths containing "boom" after the first call.
type failFS struct{ vfs.FS }

func (f failFS) Create(p string) (io.WriteCloser, error) {
	if strings.Contains(p, "Layer/L01/1.tif") {
		return nil, errors.New("disk full (injected)")
	}
	return f.FS.Create(p)
}

func TestFailedCopyLeavesNoHalfModel(t *testing.T) {
	env, _ := world(t)
	testkit.AddModel(t, env[ref("A", "FM1")], "m1", "NEW")
	dst := env[ref("A", "FM2")]
	testkit.AddModel(t, dst, "m1", "OLD")
	testkit.Write(t, dst.Repo, "other/keep.txt", "keep")
	env[ref("A", "FM2")] = model.Store{Repo: failFS{dst.Repo}, Inv: dst.Inv}

	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	err = run(t, pl)
	if err == nil || !strings.Contains(err.Error(), "injected") || !strings.Contains(err.Error(), "removed") {
		t.Fatalf("want injected failure with cleanup note, got %v", err)
	}
	if testkit.Exists(dst.Repo, "m1") || testkit.Exists(dst.Repo, "m1.his") || testkit.Exists(dst.Inv, "LIGHT_SPEC/m1") {
		t.Error("a half copied model is left on the target")
	}
	if got := testkit.Read(t, dst.Repo, "other/keep.txt"); got != "keep" {
		t.Error("unrelated data lost")
	}
}

func TestSourceChangedSincePlan(t *testing.T) {
	env, _ := world(t)
	src, dst := env[ref("A", "FM1")], env[ref("A", "FM2")]
	testkit.AddModel(t, src, "m1", "NEW")
	testkit.AddModel(t, dst, "m1", "OLD")
	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	testkit.Write(t, src.Repo, "m1/extra.txt", "appeared after planning")
	if err := run(t, pl); err == nil || !strings.Contains(err.Error(), "changed after the plan") {
		t.Fatalf("want 'changed' refusal, got %v", err)
	}
	if got := testkit.Read(t, dst.Repo, "m1/2.0/Line/Top/Top_handler.txt"); got != "handler OLD" {
		t.Error("target must be untouched when the pre-check fails")
	}
}

func TestRenameSingleAndGroup(t *testing.T) {
	env, _ := world(t)
	for _, k := range machine.Kinds {
		testkit.AddModel(t, env[ref("A", k)], "Old-01", string(k))
	}
	testkit.AddModel(t, env[ref("A", "FM1")], "other", "x")

	pl, err := ops.PlanRename(enCtx(), env, ops.RenameRequest{Ref: ref("A", "FM1"), Model: "old-01", NewName: "New-01"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(t, pl); err != nil {
		t.Fatal(err)
	}
	s := env[ref("A", "FM1")]
	if !testkit.Exists(s.Repo, "New-01/2.0/Line/Top/Top_handler.txt") || testkit.Exists(s.Repo, "Old-01") {
		t.Error("FM1 repo not renamed")
	}
	if !testkit.Exists(s.Inv, "INSPECT_SPEC/New-01-00/TOP/LIGHT0/InspectionSpec.xml") || !testkit.Exists(s.Inv, "LIGHT_SPEC/New-01/LightSpec.xml") {
		t.Error("FM1 specs not renamed")
	}
	if !strings.HasPrefix(testkit.Read(t, s.Repo, "New-01.his"), "New-01,0,0,1") || testkit.Exists(s.Repo, "Old-01.his") {
		t.Error("history not renamed/rewritten")
	}
	if !testkit.Exists(env[ref("A", "FM2")].Repo, "Old-01") {
		t.Error("single rename must not touch FM2")
	}

	// group rename: FM1 no longer has Old-01 -> skipped; FM2 and BM renamed
	pl, err = ops.PlanRename(enCtx(), env, ops.RenameRequest{Mode: ops.ModeGroup, Ref: ref("A", "FM1"), Model: "Old-01", NewName: "New-01"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Units) != 2 {
		t.Fatalf("group rename units: %d", len(pl.Units))
	}
	if err := run(t, pl); err != nil {
		t.Fatal(err)
	}
	for _, k := range []machine.NodeKind{"FM2", "BM"} {
		if !testkit.Exists(env[ref("A", k)].Repo, "New-01") || testkit.Exists(env[ref("A", k)].Repo, "Old-01") {
			t.Errorf("%s not renamed", k)
		}
	}
	if !testkit.Exists(env[ref("B", "FM1")].Repo, "") { // sanity: other machine exists and is untouched
		t.Error("world broken")
	}
}

func TestRenameConflictsAndCaseOnly(t *testing.T) {
	env, _ := world(t)
	s := env[ref("A", "FM1")]
	testkit.AddModel(t, s, "abc", "1")
	testkit.AddModel(t, s, "xyz", "2")
	ctx := enCtx()
	if _, err := ops.PlanRename(ctx, env, ops.RenameRequest{Ref: ref("A", "FM1"), Model: "abc", NewName: "XYZ"}); err == nil {
		t.Error("renaming onto an existing model (other case) must be refused")
	}
	if _, err := ops.PlanRename(ctx, env, ops.RenameRequest{Ref: ref("A", "FM1"), Model: "abc", NewName: "abc"}); err == nil {
		t.Error("renaming to the same name must be refused")
	}
	if _, err := ops.PlanRename(ctx, env, ops.RenameRequest{Ref: ref("A", "FM1"), Model: "abc", NewName: "a:c"}); err == nil {
		t.Error("illegal name must be refused")
	}
	// a case-only rename is legal
	pl, err := ops.PlanRename(ctx, env, ops.RenameRequest{Ref: ref("A", "FM1"), Model: "abc", NewName: "ABC"})
	if err != nil {
		t.Fatal(err)
	}
	if err := run(t, pl); err != nil {
		t.Fatal(err)
	}
	es, _ := s.Repo.ReadDir("")
	var names []string
	for _, e := range es {
		names = append(names, e.Name)
	}
	if got := strings.Join(names, ","); got != "ABC,ABC.his,xyz,xyz.his" {
		t.Errorf("after case-only rename: %s", got)
	}
}

// renameFailFS fails the rename of one directory to force a rollback.
type renameFailFS struct{ vfs.FS }

func (f renameFailFS) Rename(a, b string) error {
	if strings.HasPrefix(b, "LIGHT_SPEC/") {
		return errors.New("locked by another process (injected)")
	}
	return f.FS.Rename(a, b)
}

func TestRenameRollsBackEverything(t *testing.T) {
	env, _ := world(t)
	for _, k := range machine.Kinds {
		testkit.AddModel(t, env[ref("A", k)], "m1", string(k))
	}
	bm := env[ref("A", "BM")]
	env[ref("A", "BM")] = model.Store{Repo: bm.Repo, Inv: renameFailFS{bm.Inv}} // last node fails

	pl, err := ops.PlanRename(enCtx(), env, ops.RenameRequest{Mode: ops.ModeGroup, Ref: ref("A", "FM1"), Model: "m1", NewName: "m2"})
	if err != nil {
		t.Fatal(err)
	}
	err = run(t, pl)
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("want rollback, got %v", err)
	}
	for _, k := range machine.Kinds {
		s := env[ref("A", k)]
		if !testkit.Exists(s.Repo, "m1/2.0/Line/Top/Top_handler.txt") || !testkit.Exists(s.Repo, "m1.his") ||
			!testkit.Exists(s.Inv, "INSPECT_SPEC/m1-00") || !testkit.Exists(s.Inv, "LIGHT_SPEC/m1") {
			t.Errorf("%s: not restored", k)
		}
		if testkit.Exists(s.Repo, "m2") || testkit.Exists(s.Repo, "m2.his") || testkit.Exists(s.Inv, "INSPECT_SPEC/m2-00") {
			t.Errorf("%s: leftovers of the new name", k)
		}
		if !strings.HasPrefix(testkit.Read(t, s.Repo, "m1.his"), "m1,0,0,1") {
			t.Errorf("%s: history content not restored", k)
		}
	}
}

func TestJobManager(t *testing.T) {
	env, _ := world(t)
	testkit.AddModel(t, env[ref("A", "FM1")], "m1", "S")
	m := ops.NewManager()
	pl, err := ops.PlanCopy(enCtx(), env, ops.CopyRequest{Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	m.Add(pl)
	jv, err := m.Start(pl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(pl.ID); err == nil {
		t.Error("a plan must start only once")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		v, _ := m.Get(jv.ID)
		if v.State != ops.StateRunning {
			if v.State != ops.StateDone || v.Progress.FilesDone != v.Progress.FilesTotal || v.Progress.FilesTotal == 0 {
				t.Fatalf("job ended as %+v", v)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := m.Start("nope"); !errors.Is(err, ops.ErrNoPlan) {
		t.Errorf("unknown plan: %v", err)
	}
}

func TestPlanLanguages(t *testing.T) {
	env, _ := world(t)
	testkit.AddModel(t, env[ref("A", "FM1")], "m1", "S")
	testkit.AddModel(t, env[ref("A", "FM2")], "m1", "OLD")
	req := ops.CopyRequest{Source: ref("A", "FM1"), Target: ref("A", "FM2"), Model: "m1"}
	titles := map[i18n.Lang]string{}
	for _, l := range i18n.All {
		pl, err := ops.PlanCopy(i18n.With(context.Background(), l), env, req)
		if err != nil {
			t.Fatal(err)
		}
		titles[l] = pl.Title
		if len(pl.Warnings) == 0 || len(pl.Units[0].Steps) == 0 {
			t.Fatal("plan is missing warnings or steps")
		}
	}
	if titles[i18n.Ko] != "복사 m1: A FM1 → A FM2" || titles[i18n.En] != "Copy m1: A FM1 → A FM2" ||
		titles[i18n.ZhHans] != "复制 m1: A FM1 → A FM2" || titles[i18n.ZhHant] != "複製 m1: A FM1 → A FM2" {
		t.Errorf("titles: %v", titles)
	}
	// no language in the context: Korean
	pl, _ := ops.PlanCopy(context.Background(), env, req)
	if pl.Title != titles[i18n.Ko] {
		t.Errorf("default language is not Korean: %q", pl.Title)
	}
	// refusal text follows the language too
	_, err := ops.PlanCopy(i18n.With(context.Background(), i18n.ZhHant), env,
		ops.CopyRequest{Source: ref("A", "FM1"), Target: ref("A", "BM"), Model: "m1"})
	if err == nil || err.Error() != "FM 與 BM 之間不允許複製" {
		t.Errorf("zh-Hant refusal: %v", err)
	}
}
