// Package capi is the whole surface of the core as seen by a native GUI: one call that takes a
// method name, a language tag and a JSON object of arguments, and answers one JSON document
// {"ok":true,"result":...} or {"ok":false,"error":"..."}. It is plain Go (no cgo) so it is unit
// tested; cmd/afvi-core-dll only marshals C strings around it.
package capi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/ops"
	"afvi/tools/core/internal/service"
	"afvi/tools/core/internal/wailsapp"
)

// Core is one initialised core.
type Core struct {
	app *wailsapp.App
	svc *service.Service
}

var (
	mu      sync.Mutex
	current *Core
)

// Init opens (or creates) the configuration and builds the core. With selftestDir set it builds
// throw-away local machines there instead and hides the real network machine.
func Init(configPath, selftestDir string, allowNetworkWrite bool) error {
	mu.Lock()
	defer mu.Unlock()
	var (
		store *machine.Store
		self  *wailsapp.Selftest
		err   error
	)
	if selftestDir != "" {
		self, store, err = wailsapp.NewSelftest(selftestDir)
	} else {
		if configPath == "" {
			if configPath, err = machine.DefaultPath(); err != nil {
				return err
			}
		}
		store, _, err = machine.Open(filepath.Clean(configPath))
	}
	if err != nil {
		return err
	}
	svc := service.New("win32", store, allowNetworkWrite)
	current = &Core{app: wailsapp.New(svc, self), svc: svc}
	return nil
}

func get() *Core {
	mu.Lock()
	defer mu.Unlock()
	return current
}

// Call runs one method. It never panics and never returns an empty string.
func Call(method, lang, argsJSON string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = fail(fmt.Errorf("internal error in %s: %v", method, r))
		}
	}()
	c := get()
	if c == nil {
		return fail(errors.New("core not initialised"))
	}
	var args map[string]json.RawMessage
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return fail(fmt.Errorf("bad arguments: %w", err))
		}
	}
	res, err := c.dispatch(method, lang, args)
	if err != nil {
		return fail(err)
	}
	b, err := json.Marshal(struct {
		OK     bool `json:"ok"`
		Result any  `json:"result"`
	}{true, res})
	if err != nil {
		return fail(err)
	}
	return string(b)
}

func fail(err error) string {
	b, _ := json.Marshal(struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  int    `json:"code"`
	}{false, err.Error(), service.Status(err)})
	return string(b)
}

func str(a map[string]json.RawMessage, k string) string {
	var s string
	if v, ok := a[k]; ok {
		_ = json.Unmarshal(v, &s)
	}
	return s
}

func into(a map[string]json.RawMessage, k string, v any) error {
	raw, ok := a[k]
	if !ok {
		return fmt.Errorf("missing argument %q", k)
	}
	return json.Unmarshal(raw, v)
}

// Methods lists every method name Call accepts (documented in afvi_core.h).
var Methods = []string{"Health", "GetConfig", "ListModels", "Targets", "CheckName", "PlanCopy", "PlanRename",
	"StartJob", "GetJob", "CancelJob", "AddMachine", "UpdateMachine", "DeleteMachine", "RestoreDefaults",
	"ExportExcel", "SelftestEnabled", "SelftestVerify"}

func (c *Core) dispatch(method, lang string, a map[string]json.RawMessage) (any, error) {
	app := c.app
	_ = context.Background
	switch method {
	case "Health":
		return app.Health(), nil
	case "GetConfig":
		return app.GetConfig(), nil
	case "ListModels":
		return app.ListModels(lang, str(a, "machine"), str(a, "node"))
	case "Targets":
		return app.Targets(lang, str(a, "machine"), str(a, "node"))
	case "CheckName":
		return app.CheckName(lang, str(a, "name")), nil
	case "PlanCopy":
		var r ops.CopyRequest
		if err := into(a, "request", &r); err != nil {
			return nil, err
		}
		return app.PlanCopy(lang, r)
	case "PlanRename":
		var r ops.RenameRequest
		if err := into(a, "request", &r); err != nil {
			return nil, err
		}
		return app.PlanRename(lang, r)
	case "StartJob":
		return app.StartJob(lang, str(a, "planId"))
	case "GetJob":
		return app.GetJob(str(a, "id"))
	case "CancelJob":
		return map[string]bool{"canceled": app.CancelJob(str(a, "id"))}, nil
	case "AddMachine", "UpdateMachine":
		var m machine.Machine
		if err := into(a, "machine", &m); err != nil {
			return nil, err
		}
		if method == "AddMachine" {
			return app.AddMachine(lang, m)
		}
		return app.UpdateMachine(lang, str(a, "name"), m)
	case "DeleteMachine":
		return map[string]bool{"ok": true}, app.DeleteMachine(lang, str(a, "name"))
	case "RestoreDefaults":
		return map[string]bool{"ok": true}, app.RestoreDefaults(lang)
	case "ExportExcel":
		return nil, app.ExportExcel(lang)
	case "SelftestEnabled":
		return app.SelftestEnabled(), nil
	case "SelftestVerify":
		return app.SelftestVerify(str(a, "step")), nil
	}
	return nil, fmt.Errorf("unknown method %q", method)
}
