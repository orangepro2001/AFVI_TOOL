//go:build windows

package vfs

import (
	"path/filepath"
	"syscall"
)

const driveRemote = 4

var procGetDriveType = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDriveTypeW")

// isNetworkPath reports whether p is on a mapped network drive.
func isNetworkPath(p string) bool {
	vol := filepath.VolumeName(p)
	if vol == "" || len(vol) != 2 {
		return false
	}
	ptr, err := syscall.UTF16PtrFromString(vol + `\`)
	if err != nil {
		return false
	}
	r, _, _ := procGetDriveType.Call(uintptr(unsafePointer(ptr)))
	return r == driveRemote
}
