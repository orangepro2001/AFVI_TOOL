package wailsapp_test

import (
	"testing"
	"time"

	"afvi/tools/core/internal/ops"
	"afvi/tools/core/internal/policy"
	"afvi/tools/core/internal/service"
	"afvi/tools/core/internal/wailsapp"
)

func newApp(t *testing.T) *wailsapp.App {
	t.Helper()
	st, store, err := wailsapp.NewSelftest(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return wailsapp.New(service.New("test", store, false), st)
}

func wait(t *testing.T, a *wailsapp.App, id string) ops.JobView {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		j, err := a.GetJob(id)
		if err != nil {
			t.Fatal(err)
		}
		if j.State != ops.StateRunning {
			return j
		}
		if time.Now().After(deadline) {
			t.Fatal("job timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// The same journey the front end script takes, through the bound methods.
func TestRPCCopyAndRename(t *testing.T) {
	a := newApp(t)
	cfg := a.GetConfig()
	if len(cfg.Machines) != 2 || cfg.Machines[0].Name != "T1" {
		t.Fatalf("selftest must hide the real machine: %+v", cfg.Machines)
	}
	list, err := a.ListModels("ko", "T1", "FM1")
	if err != nil || len(list.Models) != 3 {
		t.Fatalf("list: %v %+v", err, list)
	}
	tg, err := a.Targets("ko", "T1", "FM1")
	if err != nil || len(tg.Single) != 2 || len(tg.Group) != 1 {
		t.Fatalf("targets: %v %+v", err, tg)
	}

	pl, err := a.PlanCopy("en", ops.CopyRequest{Mode: "single", Model: "m1",
		Source: policy.Ref{Machine: "T1", Node: "FM1"}, Target: policy.Ref{Machine: "T1", Node: "FM2"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Blockers) != 0 || pl.Units[0].Steps[0].Action != "delete" {
		t.Fatalf("plan: %+v", pl)
	}
	jv, err := a.StartJob("en", pl.ID)
	if err != nil {
		t.Fatal(err)
	}
	if j := wait(t, a, jv.ID); j.State != ops.StateDone {
		t.Fatalf("copy job: %+v", j)
	}
	if p := a.SelftestVerify("copy"); p != "" {
		t.Fatal(p)
	}

	rp, err := a.PlanRename("ko", ops.RenameRequest{Mode: "group", Model: "m2", NewName: "r2",
		Ref: policy.Ref{Machine: "T1", Node: "FM1"}})
	if err != nil {
		t.Fatal(err)
	}
	jv, err = a.StartJob("ko", rp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if j := wait(t, a, jv.ID); j.State != ops.StateDone {
		t.Fatalf("rename job: %+v", j)
	}
	if p := a.SelftestVerify("rename"); p != "" {
		t.Fatal(p)
	}
}

func TestRPCErrorsAreWordedInTheCallersLanguage(t *testing.T) {
	a := newApp(t)
	_, err := a.PlanCopy("zh-Hant", ops.CopyRequest{Mode: "single", Model: "m1",
		Source: policy.Ref{Machine: "T1", Node: "FM1"}, Target: policy.Ref{Machine: "T1", Node: "BM"}})
	if err == nil || err.Error() != "FM 與 BM 之間不允許複製" {
		t.Fatalf("got %v", err)
	}
	if err := a.ExportExcel("ko"); err == nil || err.Error() != "Excel 내보내기는 아직 구현되지 않았습니다" {
		t.Fatalf("excel entry: %v", err)
	}
	if !a.SelftestEnabled() || a.DefaultLanguage() != "ko" {
		t.Fatal("defaults")
	}
}
