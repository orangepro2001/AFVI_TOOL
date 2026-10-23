// Package vfs is the only way the rest of the program touches a share.
//
// Every path handed to an FS is relative to the FS root and uses forward
// slashes. Write protection is structural: a read-only FS refuses every
// mutating call with ErrReadOnly, so the dev-phase rule "network paths are
// read only" cannot be bypassed by a bug in the planner.
package vfs

import (
	"errors"
	"io"
	"path"
	"strings"
	"time"
)

// ErrReadOnly is returned by every mutating call on a read-only FS.
var ErrReadOnly = errors.New("file system is read-only")

// Entry describes one file or directory.
type Entry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

// FS is a rooted file system.
type FS interface {
	// Describe returns a human readable root, e.g. \\192.168.1.61\PxRepository.
	Describe() string
	ReadOnly() bool

	ReadDir(p string) ([]Entry, error)
	Stat(p string) (Entry, error)
	Open(p string) (io.ReadCloser, error)

	MkdirAll(p string) error
	Create(p string) (io.WriteCloser, error)
	Remove(p string) error
	RemoveAll(p string) error
	Rename(oldp, newp string) error
	Chtimes(p string, t time.Time) error
}

// Clean returns p as a rooted-relative slash path without "." or ".." parts.
// The root itself is "".
func Clean(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean("/" + p)
	return strings.TrimPrefix(p, "/")
}

// Join joins slash path elements and cleans the result.
func Join(elem ...string) string {
	return Clean(strings.Join(elem, "/"))
}

// IsNotExist reports whether err means "no such file or directory" for any
// FS implementation.
func IsNotExist(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errNotExist) {
		return true
	}
	return isNotExistOS(err)
}

var errNotExist = errors.New("file does not exist")
