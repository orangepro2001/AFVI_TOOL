//go:build !linux && !windows

package vfs

func isNetworkPath(string) bool { return false }
