package vfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// OSFS is a directory of the local operating system. On Windows a UNC root
// (\\host\share) works unchanged because the OS resolves it.
type OSFS struct {
	root string
	ro   bool
}

// NewOS returns an FS rooted at root. When readOnly is true every mutating
// call fails with ErrReadOnly.
func NewOS(root string, readOnly bool) *OSFS {
	return &OSFS{root: root, ro: readOnly}
}

func (f *OSFS) Describe() string { return f.root }
func (f *OSFS) ReadOnly() bool   { return f.ro }

func (f *OSFS) abs(p string) string {
	c := Clean(p)
	if c == "" {
		return f.root
	}
	return filepath.Join(f.root, filepath.FromSlash(c))
}

func entryOf(fi fs.FileInfo) Entry {
	return Entry{Name: fi.Name(), IsDir: fi.IsDir(), Size: fi.Size(), ModTime: fi.ModTime()}
}

func (f *OSFS) ReadDir(p string) ([]Entry, error) {
	des, err := os.ReadDir(f.abs(p))
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // vanished while listing
			}
			return nil, err
		}
		out = append(out, entryOf(fi))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (f *OSFS) Stat(p string) (Entry, error) {
	fi, err := os.Stat(f.abs(p))
	if err != nil {
		return Entry{}, err
	}
	return entryOf(fi), nil
}

func (f *OSFS) Open(p string) (io.ReadCloser, error) { return os.Open(f.abs(p)) }

func (f *OSFS) MkdirAll(p string) error {
	if f.ro {
		return ErrReadOnly
	}
	return os.MkdirAll(f.abs(p), 0o777)
}

func (f *OSFS) Create(p string) (io.WriteCloser, error) {
	if f.ro {
		return nil, ErrReadOnly
	}
	return os.Create(f.abs(p))
}

func (f *OSFS) Remove(p string) error {
	if f.ro {
		return ErrReadOnly
	}
	return os.Remove(f.abs(p))
}

func (f *OSFS) RemoveAll(p string) error {
	if f.ro {
		return ErrReadOnly
	}
	if Clean(p) == "" {
		return errors.New("refusing to remove the root of a share")
	}
	return os.RemoveAll(f.abs(p))
}

func (f *OSFS) Rename(oldp, newp string) error {
	if f.ro {
		return ErrReadOnly
	}
	return os.Rename(f.abs(oldp), f.abs(newp))
}

func (f *OSFS) Chtimes(p string, t time.Time) error {
	if f.ro {
		return ErrReadOnly
	}
	return os.Chtimes(f.abs(p), t, t)
}

func isNotExistOS(err error) bool { return errors.Is(err, fs.ErrNotExist) }
