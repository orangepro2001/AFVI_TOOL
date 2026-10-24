//go:build cgo

// Command afvi-core-dll builds the Go core as a C library: a DLL (-buildmode=c-shared) or a static
// archive (-buildmode=c-archive). All logic lives in internal/capi; this file only moves C strings.
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"afvi/tools/core/internal/capi"
)

// afvi_init opens the configuration. configPath may be NULL (per-user default). selftestDir non-NULL
// builds throw-away local machines there (and hides the real network machine). Returns 0 or -1.
//
//export afvi_init
func afvi_init(configPath, selftestDir *C.char, allowNetworkWrite C.int) C.int {
	if err := capi.Init(C.GoString(configPath), C.GoString(selftestDir), allowNetworkWrite != 0); err != nil {
		return -1
	}
	return 0
}

// afvi_call runs a method; the UTF-8 JSON answer is malloc'ed and must be released with afvi_free.
//
//export afvi_call
func afvi_call(method, lang, argsJSON *C.char) *C.char {
	return C.CString(capi.Call(C.GoString(method), C.GoString(lang), C.GoString(argsJSON)))
}

// afvi_free releases a string returned by afvi_call.
//
//export afvi_free
func afvi_free(p *C.char) { C.free(unsafe.Pointer(p)) }

func main() {}
