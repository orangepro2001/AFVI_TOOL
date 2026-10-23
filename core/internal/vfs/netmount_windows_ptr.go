//go:build windows

package vfs

import "unsafe"

func unsafePointer(p *uint16) unsafe.Pointer { return unsafe.Pointer(p) }
