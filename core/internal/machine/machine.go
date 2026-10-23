// Package machine holds the machine configuration: named groups of an FM1,
// an FM2 and a BM node, and the file they are stored in.
package machine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/model"
	"afvi/tools/core/internal/vfs"
)

// NodeKind is FM1, FM2 or BM.
type NodeKind string

const (
	FM1 NodeKind = "FM1"
	FM2 NodeKind = "FM2"
	BM  NodeKind = "BM"
)

// Kinds lists the node kinds in display order.
var Kinds = []NodeKind{FM1, FM2, BM}

// IsFM reports whether the kind is a front machine.
func (k NodeKind) IsFM() bool { return k == FM1 || k == FM2 }

func (k NodeKind) Valid() bool { return k == FM1 || k == FM2 || k == BM }

// Node is one computer of a machine.
type Node struct {
	Host string `json:"host"`
	// RepositoryPath / InventoryPath override the default UNC roots
	// \\host\PxRepository and \\host\PxInventory. They are used to point a
	// node at a local copy during development.
	RepositoryPath string `json:"repositoryPath,omitempty"`
	InventoryPath  string `json:"inventoryPath,omitempty"`
}

// Repository is the root of the PxRepository share.
func (n Node) Repository() string {
	if n.RepositoryPath != "" {
		return n.RepositoryPath
	}
	return `\\` + n.Host + `\PxRepository`
}

// Inventory is the root of the PxInventory share.
func (n Node) Inventory() string {
	if n.InventoryPath != "" {
		return n.InventoryPath
	}
	return `\\` + n.Host + `\PxInventory`
}

// Machine is a named group FM1 + FM2 + BM.
type Machine struct {
	Name string `json:"name"`
	FM1  Node   `json:"fm1"`
	FM2  Node   `json:"fm2"`
	BM   Node   `json:"bm"`
}

// Node returns the node of the kind.
func (m Machine) Node(k NodeKind) Node {
	switch k {
	case FM1:
		return m.FM1
	case FM2:
		return m.FM2
	}
	return m.BM
}

// Settings are the program wide options.
type Settings struct {
	SMBUser     string `json:"smbUser"`
	SMBPassword string `json:"smbPassword"`
	// AllowNetworkWrite lifts the dev-phase rule that network paths are
	// read-only. It is false until the tool is released.
	AllowNetworkWrite bool `json:"allowNetworkWrite"`
}

// Config is the file content.
type Config struct {
	Version  int       `json:"version"`
	Settings Settings  `json:"settings"`
	Machines []Machine `json:"machines"`
}

const configVersion = 1

// DefaultMachine is AFVI 13: FM1 .61, FM2 .62, BM .63.
func DefaultMachine() Machine {
	return Machine{
		Name: "AFVI 13",
		FM1:  Node{Host: "192.168.1.61"},
		FM2:  Node{Host: "192.168.1.62"},
		BM:   Node{Host: "192.168.1.63"},
	}
}

// Default is the configuration written on first start.
func Default() Config {
	return Config{
		Version:  configVersion,
		Settings: Settings{SMBUser: "pixel", SMBPassword: ""},
		Machines: []Machine{DefaultMachine()},
	}
}

// Validate checks one machine definition.
func (m Machine) Validate(tr i18n.T) error {
	if strings.TrimSpace(m.Name) == "" || m.Name != strings.TrimSpace(m.Name) {
		return errors.New(tr("m.name_empty"))
	}
	for _, k := range Kinds {
		n := m.Node(k)
		if strings.TrimSpace(n.Host) == "" && n.RepositoryPath == "" {
			return errors.New(tr("m.node_need", k))
		}
	}
	return nil
}

// Validate checks the whole configuration.
func (c Config) Validate(tr i18n.T) error {
	seen := map[string]bool{}
	for _, m := range c.Machines {
		if err := m.Validate(tr); err != nil {
			return errors.New(tr("m.in_machine", m.Name, err))
		}
		k := strings.ToLower(m.Name)
		if seen[k] {
			return errors.New(tr("m.dup", m.Name))
		}
		seen[k] = true
	}
	return nil
}

// Find returns the machine called name (case-insensitive).
func (c Config) Find(name string) (Machine, bool) {
	for _, m := range c.Machines {
		if strings.EqualFold(m.Name, name) {
			return m, true
		}
	}
	return Machine{}, false
}

// Store is a configuration file with a lock; safe for concurrent use.
type Store struct {
	path string
	mu   sync.RWMutex
	cfg  Config
}

// Open reads the file, or creates it with the defaults when it does not
// exist. created reports the latter.
func Open(path string) (s *Store, created bool, err error) {
	s = &Store{path: path}
	b, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.cfg = Default()
		if err := s.save(); err != nil {
			return nil, false, err
		}
		return s, true, nil
	case err != nil:
		return nil, false, err
	}
	if err := json.Unmarshal(b, &s.cfg); err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	if err := s.cfg.Validate(i18n.For(i18n.En)); err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	return s, false, nil
}

// Path is the file the store writes to.
func (s *Store) Path() string { return s.path }

// Get returns a copy of the configuration.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.cfg
	c.Machines = append([]Machine(nil), s.cfg.Machines...)
	return c
}

// Update applies fn to a copy of the configuration, validates and saves it.
func (s *Store) Update(tr i18n.T, fn func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cfg
	c.Machines = append([]Machine(nil), s.cfg.Machines...)
	if err := fn(&c); err != nil {
		return err
	}
	if err := c.Validate(tr); err != nil {
		return err
	}
	old := s.cfg
	s.cfg = c
	if err := s.save(); err != nil {
		s.cfg = old
		return err
	}
	return nil
}

// save writes the file atomically (temp file + rename).
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// DefaultPath is the per-user config file location.
func DefaultPath() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "AFVI_TOOLS", "config.json"), nil
}

// Opener builds the model.Store of a node, reusing connections.
type Opener struct {
	mu    sync.Mutex
	cache map[string]vfs.FS
}

// NewOpener returns an empty Opener.
func NewOpener() *Opener { return &Opener{cache: map[string]vfs.FS{}} }

func (o *Opener) open(spec string, so vfs.Options) (vfs.FS, error) {
	key := fmt.Sprintf("%s|%s|%s|%v", spec, so.SMBUser, so.SMBPassword, so.AllowNetworkWrite)
	o.mu.Lock()
	defer o.mu.Unlock()
	if f, ok := o.cache[key]; ok {
		return f, nil
	}
	f, err := vfs.Open(spec, so)
	if err != nil {
		return nil, err
	}
	o.cache[key] = f
	return f, nil
}

// Store opens the two shares of a node.
func (o *Opener) Store(n Node, set Settings) (model.Store, error) {
	so := vfs.Options{SMBUser: set.SMBUser, SMBPassword: set.SMBPassword, AllowNetworkWrite: set.AllowNetworkWrite}
	r, err := o.open(n.Repository(), so)
	if err != nil {
		return model.Store{}, err
	}
	i, err := o.open(n.Inventory(), so)
	if err != nil {
		return model.Store{}, err
	}
	return model.Store{Repo: r, Inv: i}, nil
}

// LocalMachine returns a machine whose three nodes live in dir/FM1, dir/FM2
// and dir/BM (each with PxRepository and PxInventory). It is the safe place
// to try copies during development: these paths are writable.
func LocalMachine(name, dir string) Machine {
	node := func(k NodeKind) Node {
		d := filepath.Join(dir, string(k))
		return Node{Host: "local", RepositoryPath: filepath.Join(d, "PxRepository"), InventoryPath: filepath.Join(d, "PxInventory")}
	}
	return Machine{Name: name, FM1: node(FM1), FM2: node(FM2), BM: node(BM)}
}

// CreateDirs makes the share directories of every node that has a local path.
func (m Machine) CreateDirs() error {
	for _, k := range Kinds {
		n := m.Node(k)
		for _, p := range []string{n.RepositoryPath, n.InventoryPath} {
			if p == "" {
				continue
			}
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
		}
	}
	return nil
}
