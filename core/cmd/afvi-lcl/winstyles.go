//go:build windows && winstyles

package main

// Built with -tags winstyles the Windows executable carries the comctl32 v6 manifest of golcl (visual styles on:
// the Windows 10/11 look). Without the tag there is no manifest and the stock controls draw in the classic style.
import _ "github.com/energye/golcl/pkgs/winappres"
