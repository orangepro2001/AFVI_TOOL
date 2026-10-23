package vfs

import (
	"fmt"
	"runtime"
	"strings"
)

// Options controls how a root specification is opened.
type Options struct {
	// SMBUser / SMBPassword are used for \\host\share roots on systems that
	// cannot resolve UNC paths themselves (Linux, macOS). On Windows the
	// logon session of the user is used instead.
	SMBUser     string
	SMBPassword string
	// AllowNetworkWrite lifts the dev-phase lock. Without it every root that
	// is a network location is opened read-only.
	AllowNetworkWrite bool
}

// IsUNC reports whether spec is a \\host\share[\sub] or //host/share[/sub] path.
func IsUNC(spec string) bool {
	return strings.HasPrefix(spec, `\\`) || strings.HasPrefix(spec, "//")
}

// SplitUNC splits a UNC path into host, share and the remaining sub path.
func SplitUNC(spec string) (host, share, sub string, err error) {
	if !IsUNC(spec) {
		return "", "", "", fmt.Errorf("not a UNC path: %q", spec)
	}
	parts := strings.Split(strings.Trim(strings.ReplaceAll(spec, `\`, "/"), "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", fmt.Errorf("UNC path needs host and share: %q", spec)
	}
	return parts[0], parts[1], strings.Join(parts[2:], "/"), nil
}

// Open turns a root specification into an FS.
//
//   - UNC path on Windows: OS file system, read-only unless the lock is lifted.
//   - UNC path elsewhere: SMB client, always read-only.
//   - Local path: OS file system; read-only when the path lies on a network
//     mount and the lock is not lifted.
func Open(spec string, o Options) (FS, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, fmt.Errorf("empty root path")
	}
	if IsUNC(spec) {
		if runtime.GOOS == "windows" {
			return NewOS(spec, !o.AllowNetworkWrite), nil
		}
		host, share, sub, err := SplitUNC(spec)
		if err != nil {
			return nil, err
		}
		return NewSMB(host, share, sub, o.SMBUser, o.SMBPassword), nil
	}
	ro := false
	if !o.AllowNetworkWrite && isNetworkPath(spec) {
		ro = true
	}
	return NewOS(spec, ro), nil
}
