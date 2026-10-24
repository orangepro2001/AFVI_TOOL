package wailsapp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
)

// Selftest drives the real window: with -selftest the program builds throw-away local
// machines, the front end script (selftest.js) clicks through the UI, and the Go side
// checks the files afterwards. Exit code 0 = every step passed.
type Selftest struct {
	Dir string
	mu  sync.Mutex
	// Done receives the exit code once the front end has reported.
	Done chan int
}

// NewSelftest prepares fixtures below dir and returns the config store to run with.
func NewSelftest(dir string) (*Selftest, *machine.Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	store, _, err := machine.Open(filepath.Join(dir, "config.json"))
	if err != nil {
		return nil, nil, err
	}
	tr := i18n.For(i18n.En)
	t1, t2 := machine.LocalMachine("T1", filepath.Join(dir, "T1")), machine.LocalMachine("T2", filepath.Join(dir, "T2"))
	for _, m := range []machine.Machine{t1, t2} {
		if err := m.CreateDirs(); err != nil {
			return nil, nil, err
		}
	}
	err = store.Update(tr, func(c *machine.Config) error {
		c.Machines = []machine.Machine{t1, t2} // never touch the real network machine
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	st := &Selftest{Dir: dir, Done: make(chan int, 1)}
	add := func(m machine.Machine, node machine.NodeKind, name, tag string) error {
		n := m.Node(node)
		files := map[string]string{
			filepath.Join(n.RepositoryPath, name, "2.0", "Line", "Top", "Top_handler.txt"): "handler " + tag,
			filepath.Join(n.RepositoryPath, name+".his"):                                   name + ",0,0,1,Admin,20260507114036\r\n",
			filepath.Join(n.InventoryPath, "INSPECT_SPEC", name+"-00", "TOP", "a.xml"):     "spec " + tag,
			filepath.Join(n.InventoryPath, "LIGHT_SPEC", name, "LightSpec.xml"):            "light " + tag,
		}
		for p, c := range files {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(p, []byte(c), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
	for _, f := range []struct {
		m    machine.Machine
		n    machine.NodeKind
		name string
		tag  string
	}{
		{t1, machine.FM1, "m1", "NEW"}, {t1, machine.FM1, "m2", "TWO"}, {t1, machine.FM1, "7M99-535", "X"},
		{t1, machine.FM2, "M1", "OLD"}, // same name in another case: the copy must replace it
		{t1, machine.BM, "m2", "BM-TWO"},
	} {
		if err := add(f.m, f.n, f.name, f.tag); err != nil {
			return nil, nil, err
		}
	}
	return st, store, nil
}

func (s *Selftest) read(machineName string, node machine.NodeKind, rel string) (string, error) {
	// repository root of a local machine is <dir>/<machine>/<node>/PxRepository
	p := filepath.Join(s.Dir, machineName, string(node), "PxRepository", filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	return string(b), err
}

func (s *Selftest) exists(machineName string, node machine.NodeKind, share, rel string) bool {
	_, err := os.Stat(filepath.Join(s.Dir, machineName, string(node), share, filepath.FromSlash(rel)))
	return err == nil
}

// Enabled reports whether the front end should run its self-test script.
func (a *App) SelftestEnabled() bool { return a.self != nil }

// SelftestVerify checks the files after a scripted step; it returns "" when fine.
func (a *App) SelftestVerify(step string) string {
	s := a.self
	if s == nil {
		return "selftest not enabled"
	}
	var problems []string
	need := func(ok bool, what string) {
		if !ok {
			problems = append(problems, what)
		}
	}
	switch step {
	case "copy": // m1 copied FM1 -> FM2, replacing "M1"
		got, err := s.read("T1", machine.FM2, "m1/2.0/Line/Top/Top_handler.txt")
		need(err == nil && got == "handler NEW", fmt.Sprintf("FM2 m1 content %q (%v)", got, err))
		need(!s.exists("T1", machine.FM2, "PxRepository", "M1"), "old M1 directory is still there")
		his, _ := s.read("T1", machine.FM2, "m1.his")
		need(strings.HasPrefix(his, "m1,"), "history not copied: "+his)
		need(s.exists("T1", machine.FM2, "PxInventory", "INSPECT_SPEC/m1-00"), "inspect spec missing")
		need(s.exists("T1", machine.FM1, "PxRepository", "m1"), "source disappeared")
	case "rename": // m2 renamed to r2 on FM1 and BM (group), FM2 has no m2
		for _, n := range []machine.NodeKind{machine.FM1, machine.BM} {
			need(s.exists("T1", n, "PxRepository", "r2/2.0"), string(n)+": r2 missing")
			need(!s.exists("T1", n, "PxRepository", "m2"), string(n)+": m2 still there")
			his, _ := s.read("T1", n, "r2.his")
			need(strings.HasPrefix(his, "r2,"), string(n)+": history not rewritten: "+his)
		}
	default:
		return "unknown step " + step
	}
	return strings.Join(problems, "; ")
}

// SelftestReport ends the run: the front end calls it once with the verdict.
func (a *App) SelftestReport(ok bool, message string) {
	s := a.self
	if s == nil {
		return
	}
	code := 0
	if ok {
		fmt.Println("SELFTEST OK " + message)
	} else {
		fmt.Fprintln(os.Stderr, "SELFTEST FAIL: "+message)
		code = 1
	}
	select {
	case s.Done <- code:
	default:
	}
}
