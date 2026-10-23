package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"afvi/tools/core/internal/vfs"
)

// Share names the two shares of a node.
type Share string

const (
	ShareRepo Share = "PxRepository"
	ShareInv  Share = "PxInventory"
)

// Kind is the role of a part inside a model bundle.
type Kind string

const (
	KindRepo    Kind = "repository"
	KindHistory Kind = "history"
	KindInspect Kind = "inspect"
	KindLight   Kind = "light"
)

const (
	inspectDir = "INSPECT_SPEC"
	lightDir   = "LIGHT_SPEC"
	hisExt     = ".his"
)

// Store is one node: its PxRepository and PxInventory shares.
type Store struct {
	Repo vfs.FS
	Inv  vfs.FS
}

// FS returns the file system of a share.
func (s Store) FS(sh Share) vfs.FS {
	if sh == ShareRepo {
		return s.Repo
	}
	return s.Inv
}

// ReadOnly is true when any share of the node refuses writes.
func (s Store) ReadOnly() bool { return s.Repo.ReadOnly() || s.Inv.ReadOnly() }

// Part is one directory or file that belongs to a model.
type Part struct {
	Kind  Kind   `json:"kind"`
	Share Share  `json:"share"`
	Dir   string `json:"dir"`  // parent directory inside the share, "" = share root
	Name  string `json:"name"` // actual entry name (original case)
	IsDir bool   `json:"isDir"`
	// Suffix is "-00" for INSPECT_SPEC parts, "" otherwise.
	Suffix  string    `json:"suffix,omitempty"`
	ModTime time.Time `json:"modTime"`
}

// Path is the slash path of the part inside its share.
func (p Part) Path() string { return vfs.Join(p.Dir, p.Name) }

// Bundle is everything that belongs to one model on one node.
type Bundle struct {
	Name  string `json:"name"` // canonical name: the repository directory when present
	Class string `json:"class"`
	Parts []Part `json:"parts"`
}

// Has reports whether the bundle contains a part of the kind.
func (b Bundle) Has(k Kind) bool {
	for _, p := range b.Parts {
		if p.Kind == k {
			return true
		}
	}
	return false
}

// Count returns how many parts have the kind.
func (b Bundle) Count(k Kind) int {
	n := 0
	for _, p := range b.Parts {
		if p.Kind == k {
			n++
		}
	}
	return n
}

// ModTime is the newest modification time among the parts.
func (b Bundle) ModTime() time.Time {
	var t time.Time
	for _, p := range b.Parts {
		if p.ModTime.After(t) {
			t = p.ModTime
		}
	}
	return t
}

// splitInspect splits "21csp043-09-00" into "21csp043-09" and "-00".
func splitInspect(name string) (base, suffix string, ok bool) {
	n := len(name)
	if n <= 3 || name[n-3] != '-' {
		return "", "", false
	}
	if d1, d2 := name[n-2], name[n-1]; d1 < '0' || d1 > '9' || d2 < '0' || d2 > '9' {
		return "", "", false
	}
	return name[:n-3], name[n-3:], true
}

// listAll collects every managed part of the node, grouped by model key.
func listAll(s Store) (map[string][]Part, error) {
	groups := map[string][]Part{}
	add := func(key string, p Part) { groups[key] = append(groups[key], p) }

	es, err := s.Repo.ReadDir("")
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.Repo.Describe(), err)
	}
	for _, e := range es {
		switch {
		case e.IsDir:
			add(Key(e.Name), Part{Kind: KindRepo, Share: ShareRepo, Name: e.Name, IsDir: true, ModTime: e.ModTime})
		case len(e.Name) > len(hisExt) && strings.EqualFold(e.Name[len(e.Name)-len(hisExt):], hisExt):
			stem := e.Name[:len(e.Name)-len(hisExt)]
			add(Key(stem), Part{Kind: KindHistory, Share: ShareRepo, Name: e.Name, ModTime: e.ModTime})
		}
	}

	ies, err := s.Inv.ReadDir(inspectDir)
	if err != nil && !vfs.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s/%s: %w", s.Inv.Describe(), inspectDir, err)
	}
	for _, e := range ies {
		if !e.IsDir {
			continue
		}
		if base, suf, ok := splitInspect(e.Name); ok {
			add(Key(base), Part{Kind: KindInspect, Share: ShareInv, Dir: inspectDir, Name: e.Name, IsDir: true, Suffix: suf, ModTime: e.ModTime})
		}
	}

	les, err := s.Inv.ReadDir(lightDir)
	if err != nil && !vfs.IsNotExist(err) {
		return nil, fmt.Errorf("reading %s/%s: %w", s.Inv.Describe(), lightDir, err)
	}
	for _, e := range les {
		if e.IsDir {
			add(Key(e.Name), Part{Kind: KindLight, Share: ShareInv, Dir: lightDir, Name: e.Name, IsDir: true, ModTime: e.ModTime})
		}
	}
	return groups, nil
}

func bundleOf(parts []Part) Bundle {
	sort.SliceStable(parts, func(i, j int) bool {
		if parts[i].Kind != parts[j].Kind {
			return kindOrder(parts[i].Kind) < kindOrder(parts[j].Kind)
		}
		return parts[i].Name < parts[j].Name
	})
	name := ""
	for _, k := range []Kind{KindRepo, KindLight, KindInspect, KindHistory} {
		for _, p := range parts {
			if p.Kind != k {
				continue
			}
			switch k {
			case KindInspect:
				name = p.Name[:len(p.Name)-len(p.Suffix)]
			case KindHistory:
				name = p.Name[:len(p.Name)-len(hisExt)]
			default:
				name = p.Name
			}
			break
		}
		if name != "" {
			break
		}
	}
	return Bundle{Name: name, Class: Classify(name), Parts: parts}
}

func kindOrder(k Kind) int {
	switch k {
	case KindRepo:
		return 0
	case KindHistory:
		return 1
	case KindInspect:
		return 2
	}
	return 3
}

// Scan lists every model of the node, sorted by name.
func Scan(s Store) ([]Bundle, error) {
	groups, err := listAll(s)
	if err != nil {
		return nil, err
	}
	out := make([]Bundle, 0, len(groups))
	for _, parts := range groups {
		out = append(out, bundleOf(parts))
	}
	sort.Slice(out, func(i, j int) bool { return Key(out[i].Name) < Key(out[j].Name) })
	return out, nil
}

// Locate returns the bundle of one model (case-insensitive). Found is false
// when the node has no part of it at all.
func Locate(s Store, name string) (b Bundle, found bool, err error) {
	groups, err := listAll(s)
	if err != nil {
		return Bundle{}, false, err
	}
	parts, ok := groups[Key(name)]
	if !ok {
		return Bundle{Name: name, Class: Classify(name)}, false, nil
	}
	return bundleOf(parts), true, nil
}

// TargetNames returns the entry names a model called name would have for the
// given source bundle: the repository dir, the history file, the inspect
// directories (suffix kept) and the light directory.
func (p Part) TargetName(newName string) string {
	switch p.Kind {
	case KindHistory:
		return newName + hisExt
	case KindInspect:
		return newName + p.Suffix
	}
	return newName
}

// Belongs reports whether the part is one of the entries a model called name
// consists of. Destructive code calls it before every delete.
func (p Part) Belongs(name string) bool {
	switch p.Kind {
	case KindRepo:
		return p.Share == ShareRepo && p.Dir == "" && Equal(p.Name, name)
	case KindHistory:
		return p.Share == ShareRepo && p.Dir == "" && Equal(p.Name, name+hisExt)
	case KindInspect:
		base, suf, ok := splitInspect(p.Name)
		return ok && suf == p.Suffix && p.Share == ShareInv && p.Dir == inspectDir && Equal(base, name)
	case KindLight:
		return p.Share == ShareInv && p.Dir == lightDir && Equal(p.Name, name)
	}
	return false
}
