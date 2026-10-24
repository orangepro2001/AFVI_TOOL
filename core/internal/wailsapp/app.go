// Package wailsapp is the object the Wails front end calls (in-process RPC): a thin
// layer over service.Service. Every method that produces user-visible text takes the
// UI language as its first argument, because there are no HTTP headers here. The
// package has no Wails import, so it is tested like any other Go package.
package wailsapp

import (
	"context"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/ops"
	"afvi/tools/core/internal/service"
)

// App is bound to the front end; its exported methods become window.go.main.App.*.
type App struct {
	svc  *service.Service
	self *Selftest // nil unless started with -selftest
	// Quit is called by the front end to close the window (set by main).
	Quit func()
}

// New creates the app object.
func New(svc *service.Service, self *Selftest) *App { return &App{svc: svc, self: self} }

func ctx(lang string) context.Context { return i18n.With(context.Background(), i18n.Parse(lang)) }

func (a *App) Health() service.Health        { return a.svc.Health(context.Background()) }
func (a *App) GetConfig() service.ConfigView { return a.svc.GetConfig(context.Background()) }
func (a *App) DefaultLanguage() string       { return string(i18n.Default) }
func (a *App) Languages() []string {
	out := make([]string, 0, len(i18n.All))
	for _, l := range i18n.All {
		out = append(out, string(l))
	}
	return out
}

func (a *App) AddMachine(lang string, m machine.Machine) (machine.Machine, error) {
	return a.svc.AddMachine(ctx(lang), m)
}

func (a *App) UpdateMachine(lang, name string, m machine.Machine) (machine.Machine, error) {
	return a.svc.UpdateMachine(ctx(lang), name, m)
}

func (a *App) DeleteMachine(lang, name string) error { return a.svc.DeleteMachine(ctx(lang), name) }
func (a *App) RestoreDefaults(lang string) error     { return a.svc.RestoreDefaults(ctx(lang)) }

func (a *App) ListModels(lang, machineName, node string) (service.ModelList, error) {
	return a.svc.ListModels(ctx(lang), machineName, node)
}

func (a *App) Targets(lang, machineName, node string) (service.TargetList, error) {
	return a.svc.Targets(ctx(lang), machineName, node)
}

func (a *App) CheckName(lang, name string) service.NameCheck { return a.svc.CheckName(ctx(lang), name) }

func (a *App) PlanCopy(lang string, req ops.CopyRequest) (*ops.Plan, error) {
	return a.svc.PlanCopy(ctx(lang), req)
}

func (a *App) PlanRename(lang string, req ops.RenameRequest) (*ops.Plan, error) {
	return a.svc.PlanRename(ctx(lang), req)
}

func (a *App) StartJob(lang, planID string) (ops.JobView, error) {
	return a.svc.StartJob(ctx(lang), planID)
}
func (a *App) GetJob(id string) (ops.JobView, error) { return a.svc.GetJob(context.Background(), id) }
func (a *App) CancelJob(id string) bool              { return a.svc.CancelJob(context.Background(), id) }
func (a *App) ExportExcel(lang string) error         { return a.svc.ExportExcel(ctx(lang)) }

// CloseWindow lets the drawn caption bar's close button end the program.
func (a *App) CloseWindow() {
	if a.Quit != nil {
		a.Quit()
	}
}
