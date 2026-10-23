package vfs_test

import "runtime"

func hasWindows() bool { return runtime.GOOS == "windows" }
