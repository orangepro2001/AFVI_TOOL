package machine_test

import (
	"os"
	"path/filepath"
	"testing"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
)

var tr = i18n.For(i18n.En)

func TestFirstStartWritesDefaultMachine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	s, created, err := machine.Open(path)
	if err != nil || !created {
		t.Fatalf("Open: created=%v err=%v", created, err)
	}
	m, ok := s.Get().Find("afvi 13")
	if !ok {
		t.Fatal("default machine AFVI 13 missing")
	}
	if m.FM1.Host != "192.168.1.61" || m.FM2.Host != "192.168.1.62" || m.BM.Host != "192.168.1.63" {
		t.Errorf("default hosts: %+v", m)
	}
	if got := m.FM1.Repository(); got != `\\192.168.1.61\PxRepository` {
		t.Errorf("repository: %s", got)
	}
	if got := m.BM.Inventory(); got != `\\192.168.1.63\PxInventory` {
		t.Errorf("inventory: %s", got)
	}
	if set := s.Get().Settings; set.SMBUser != "pixel" || set.SMBPassword != "" || set.AllowNetworkWrite {
		t.Errorf("settings: %+v", set)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("file not written")
	}
}

func TestSecondStartKeepsUserEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, _, _ := machine.Open(path)
	if err := s.Update(tr, func(c *machine.Config) error {
		c.Machines = append(c.Machines, machine.Machine{Name: "AFVI 14",
			FM1: machine.Node{Host: "10.0.0.1"}, FM2: machine.Node{Host: "10.0.0.2"}, BM: machine.Node{Host: "10.0.0.3"}})
		c.Machines = c.Machines[1:] // the user removed AFVI 13
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s2, created, err := machine.Open(path)
	if err != nil || created {
		t.Fatalf("reopen: created=%v err=%v", created, err)
	}
	c := s2.Get()
	if len(c.Machines) != 1 || c.Machines[0].Name != "AFVI 14" {
		t.Errorf("user edit lost: %+v", c.Machines)
	}
}

func TestValidation(t *testing.T) {
	s, _, _ := machine.Open(filepath.Join(t.TempDir(), "c.json"))
	dup := machine.DefaultMachine()
	dup.Name = "afvi 13"
	if err := s.Update(tr, func(c *machine.Config) error { c.Machines = append(c.Machines, dup); return nil }); err == nil {
		t.Error("duplicate (case-insensitive) name accepted")
	}
	noHost := machine.Machine{Name: "X", FM1: machine.Node{Host: "a"}, FM2: machine.Node{Host: "b"}}
	if err := s.Update(tr, func(c *machine.Config) error { c.Machines = append(c.Machines, noHost); return nil }); err == nil {
		t.Error("machine without BM address accepted")
	}
	if len(s.Get().Machines) != 1 {
		t.Error("a rejected update changed the config")
	}
	local := machine.Node{RepositoryPath: "/tmp/x/PxRepository", InventoryPath: "/tmp/x/PxInventory"}
	if local.Repository() != "/tmp/x/PxRepository" || local.Inventory() != "/tmp/x/PxInventory" {
		t.Error("path overrides ignored")
	}
}

func TestLocalMachine(t *testing.T) {
	dir := t.TempDir()
	m := machine.LocalMachine("LOCAL TEST", dir)
	if err := m.Validate(tr); err != nil {
		t.Fatal(err)
	}
	if err := m.CreateDirs(); err != nil {
		t.Fatal(err)
	}
	for _, k := range machine.Kinds {
		for _, p := range []string{m.Node(k).Repository(), m.Node(k).Inventory()} {
			if st, err := os.Stat(p); err != nil || !st.IsDir() {
				t.Errorf("%s: %v", p, err)
			}
		}
	}
	if m.FM1.Repository() != filepath.Join(dir, "FM1", "PxRepository") {
		t.Errorf("FM1 repository: %s", m.FM1.Repository())
	}
}
