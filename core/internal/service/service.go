// Package service is the application's use-case layer: everything the GUI can ask
// of the core, as plain Go methods with a context (carrying the UI language) and
// plain data in and out. It knows nothing about transports: package server wraps
// it in HTTP/JSON for a separate GUI process, and the Wails app binds it directly
// to the front end (in-process RPC, no port, no token).
package service

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"time"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/model"
	"afvi/tools/core/internal/ops"
	"afvi/tools/core/internal/policy"
)

// Error is an error with the HTTP status a transport should use for it. The
// message is already in the user's language.
type Error struct {
	Status int
	Msg    string
}

func (e *Error) Error() string { return e.Msg }

// Status returns the status code of err: that of a *Error, 400 for any other.
func Status(err error) int {
	var se *Error
	if errors.As(err, &se) {
		return se.Status
	}
	return http.StatusBadRequest
}

func bad(err error) error { return &Error{Status: http.StatusBadRequest, Msg: err.Error()} }

// Service holds the state behind the use cases.
type Service struct {
	Version string
	cfg     *machine.Store
	opener  *machine.Opener
	jobs    *ops.Manager
	// forceWrite is the -allow-network-write flag: it lifts the dev-phase lock
	// regardless of the configuration file.
	forceWrite bool
}

// New creates the service. forceWrite lifts the read-only lock on network paths.
func New(version string, cfg *machine.Store, forceWrite bool) *Service {
	return &Service{Version: version, cfg: cfg, opener: machine.NewOpener(), jobs: ops.NewManager(), forceWrite: forceWrite}
}

// Config is the configuration store (for paths and tests).
func (s *Service) Config() *machine.Store { return s.cfg }

func (s *Service) settings() machine.Settings {
	set := s.cfg.Get().Settings
	if s.forceWrite {
		set.AllowNetworkWrite = true
	}
	return set
}

// Store implements ops.Env: it opens the shares of a node.
func (s *Service) Store(ctx context.Context, ref policy.Ref) (model.Store, error) {
	tr := i18n.FromContext(ctx)
	c := s.cfg.Get()
	m, ok := c.Find(ref.Machine)
	if !ok {
		return model.Store{}, bad(errors.New(tr("m.not_found", ref.Machine)))
	}
	if !ref.Node.Valid() {
		return model.Store{}, bad(errors.New(tr("m.bad_node", ref.Node)))
	}
	return s.opener.Store(m.Node(ref.Node), s.settings())
}

// ---- health / configuration --------------------------------------------------

// Health describes the running core.
type Health struct {
	Version           string `json:"version"`
	OS                string `json:"os"`
	AllowNetworkWrite bool   `json:"allowNetworkWrite"`
	ConfigPath        string `json:"configPath"`
}

func (s *Service) Health(context.Context) Health {
	return Health{Version: s.Version, OS: runtime.GOOS, AllowNetworkWrite: s.settings().AllowNetworkWrite, ConfigPath: s.cfg.Path()}
}

// SettingsView is the part of the settings the GUI may see (never the password).
type SettingsView struct {
	SMBUser           string `json:"smbUser"`
	AllowNetworkWrite bool   `json:"allowNetworkWrite"`
}

// ConfigView is the machines and settings.
type ConfigView struct {
	Machines []machine.Machine  `json:"machines"`
	Settings SettingsView       `json:"settings"`
	Kinds    []machine.NodeKind `json:"kinds"`
}

func (s *Service) GetConfig(context.Context) ConfigView {
	c := s.cfg.Get()
	return ConfigView{Machines: c.Machines,
		Settings: SettingsView{SMBUser: c.Settings.SMBUser, AllowNetworkWrite: s.settings().AllowNetworkWrite}, Kinds: machine.Kinds}
}

func (s *Service) AddMachine(ctx context.Context, m machine.Machine) (machine.Machine, error) {
	tr := i18n.FromContext(ctx)
	err := s.cfg.Update(tr, func(c *machine.Config) error {
		if _, ok := c.Find(m.Name); ok {
			return errors.New(tr("m.exists", m.Name))
		}
		c.Machines = append(c.Machines, m)
		return nil
	})
	if err != nil {
		return machine.Machine{}, bad(err)
	}
	return m, nil
}

func (s *Service) UpdateMachine(ctx context.Context, name string, m machine.Machine) (machine.Machine, error) {
	tr := i18n.FromContext(ctx)
	err := s.cfg.Update(tr, func(c *machine.Config) error {
		for i := range c.Machines {
			if strings.EqualFold(c.Machines[i].Name, name) {
				c.Machines[i] = m
				return nil
			}
		}
		return errors.New(tr("m.not_found", name))
	})
	if err != nil {
		return machine.Machine{}, bad(err)
	}
	return m, nil
}

func (s *Service) DeleteMachine(ctx context.Context, name string) error {
	tr := i18n.FromContext(ctx)
	return bad2(s.cfg.Update(tr, func(c *machine.Config) error {
		for i := range c.Machines {
			if strings.EqualFold(c.Machines[i].Name, name) {
				c.Machines = append(c.Machines[:i], c.Machines[i+1:]...)
				return nil
			}
		}
		return errors.New(tr("m.not_found", name))
	}))
}

// RestoreDefaults puts the built-in AFVI 13 back when it is missing.
func (s *Service) RestoreDefaults(ctx context.Context) error {
	return bad2(s.cfg.Update(i18n.FromContext(ctx), func(c *machine.Config) error {
		d := machine.DefaultMachine()
		if _, ok := c.Find(d.Name); !ok {
			c.Machines = append(c.Machines, d)
		}
		return nil
	}))
}

func bad2(err error) error {
	if err == nil {
		return nil
	}
	return bad(err)
}

// ---- models ----------------------------------------------------------------

// ModelRow is one line of the model list.
type ModelRow struct {
	Name    string `json:"name"`
	Class   string `json:"class"`
	Repo    bool   `json:"repo"`
	History bool   `json:"history"`
	Inspect int    `json:"inspect"`
	Light   bool   `json:"light"`
	ModTime string `json:"modTime"`
}

// ModelList is the scan of one node.
type ModelList struct {
	Models     []ModelRow `json:"models"`
	ReadOnly   bool       `json:"readOnly"`
	Repository string     `json:"repository"`
	Inventory  string     `json:"inventory"`
}

func (s *Service) ListModels(ctx context.Context, machineName, node string) (ModelList, error) {
	ref := policy.Ref{Machine: machineName, Node: machine.NodeKind(strings.ToUpper(node))}
	st, err := s.Store(ctx, ref)
	if err != nil {
		return ModelList{}, err
	}
	bundles, err := model.Scan(st)
	if err != nil {
		return ModelList{}, &Error{Status: http.StatusBadGateway, Msg: err.Error()}
	}
	rows := make([]ModelRow, 0, len(bundles))
	for _, b := range bundles {
		row := ModelRow{Name: b.Name, Class: b.Class, Repo: b.Has(model.KindRepo), History: b.Has(model.KindHistory),
			Inspect: b.Count(model.KindInspect), Light: b.Has(model.KindLight)}
		if t := b.ModTime(); !t.IsZero() {
			row.ModTime = t.Format(time.RFC3339)
		}
		rows = append(rows, row)
	}
	return ModelList{Models: rows, ReadOnly: st.ReadOnly(), Repository: st.Repo.Describe(), Inventory: st.Inv.Describe()}, nil
}

// TargetList is what the rules allow for one source node.
type TargetList struct {
	Single []policy.Ref `json:"single"`
	Group  []string     `json:"group"`
}

func (s *Service) Targets(ctx context.Context, machineName, node string) (TargetList, error) {
	tr := i18n.FromContext(ctx)
	src := policy.Ref{Machine: machineName, Node: machine.NodeKind(strings.ToUpper(node))}
	c := s.cfg.Get()
	if _, ok := c.Find(src.Machine); !ok {
		return TargetList{}, bad(errors.New(tr("m.not_found", src.Machine)))
	}
	var names []string
	for _, m := range c.Machines {
		names = append(names, m.Name)
	}
	group := []string{}
	for _, n := range names {
		if policy.CheckGroupCopy(tr, src.Machine, n) == nil {
			group = append(group, n)
		}
	}
	return TargetList{Single: policy.Targets(src, names), Group: group}, nil
}

// NameCheck is the verdict on a new model name.
type NameCheck struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Class string `json:"class"`
}

func (s *Service) CheckName(ctx context.Context, name string) NameCheck {
	out := NameCheck{OK: true, Class: model.Classify(name)}
	if err := model.ValidateName(i18n.FromContext(ctx), name); err != nil {
		out.OK, out.Error = false, err.Error()
	}
	return out
}

// ---- plans and jobs -----------------------------------------------------------

func (s *Service) PlanCopy(ctx context.Context, req ops.CopyRequest) (*ops.Plan, error) {
	pl, err := ops.PlanCopy(ctx, s, req)
	if err != nil {
		return nil, bad(err)
	}
	s.jobs.Add(pl)
	return pl, nil
}

func (s *Service) PlanRename(ctx context.Context, req ops.RenameRequest) (*ops.Plan, error) {
	pl, err := ops.PlanRename(ctx, s, req)
	if err != nil {
		return nil, bad(err)
	}
	s.jobs.Add(pl)
	return pl, nil
}

func (s *Service) StartJob(ctx context.Context, planID string) (ops.JobView, error) {
	tr := i18n.FromContext(ctx)
	jv, err := s.jobs.Start(planID)
	switch {
	case errors.Is(err, ops.ErrBusy):
		return ops.JobView{}, &Error{Status: http.StatusConflict, Msg: tr("j.busy")}
	case errors.Is(err, ops.ErrNoPlan):
		return ops.JobView{}, &Error{Status: http.StatusNotFound, Msg: tr("j.no_plan")}
	case err != nil:
		return ops.JobView{}, bad(err)
	}
	return jv, nil
}

func (s *Service) GetJob(_ context.Context, id string) (ops.JobView, error) {
	jv, ok := s.jobs.Get(id)
	if !ok {
		return ops.JobView{}, &Error{Status: http.StatusNotFound, Msg: "no such job"}
	}
	return jv, nil
}

func (s *Service) CancelJob(_ context.Context, id string) bool { return s.jobs.Cancel(id) }

// ExportExcel is the reserved entry of "Export Data To Excel"; not implemented yet.
func (s *Service) ExportExcel(ctx context.Context) error {
	return &Error{Status: http.StatusNotImplemented, Msg: i18n.FromContext(ctx)("s.excel_na")}
}
