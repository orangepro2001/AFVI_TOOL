// Package server is the HTTP/JSON contract between the GUI and the core.
// It listens on loopback only and every request must carry the per-launch
// token the core printed on stdout.
package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
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

// Server holds the state behind the API.
type Server struct {
	Version string
	cfg     *machine.Store
	opener  *machine.Opener
	jobs    *ops.Manager
	token   string
	// forceWrite is the -allow-network-write flag: it lifts the dev-phase
	// lock regardless of the configuration file.
	forceWrite bool
}

// New creates a server. forceWrite lifts the read-only lock on network paths.
func New(version string, cfg *machine.Store, forceWrite bool) *Server {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return &Server{Version: version, cfg: cfg, opener: machine.NewOpener(), jobs: ops.NewManager(),
		token: hex.EncodeToString(b), forceWrite: forceWrite}
}

// Token is the secret the client must send in X-Afvi-Token.
func (s *Server) Token() string { return s.token }

func (s *Server) settings() machine.Settings {
	set := s.cfg.Get().Settings
	if s.forceWrite {
		set.AllowNetworkWrite = true
	}
	return set
}

// Store implements ops.Env.
func (s *Server) Store(ctx context.Context, ref policy.Ref) (model.Store, error) {
	tr := i18n.FromContext(ctx)
	c := s.cfg.Get()
	m, ok := c.Find(ref.Machine)
	if !ok {
		return model.Store{}, errors.New(tr("m.not_found", ref.Machine))
	}
	if !ref.Node.Valid() {
		return model.Store{}, errors.New(tr("m.bad_node", ref.Node))
	}
	return s.opener.Store(m.Node(ref.Node), s.settings())
}

// Handler returns the root handler, bound to listenAddr for Host checking.
func (s *Server) Handler(listenAddr string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.health)
	mux.HandleFunc("GET /v1/config", s.getConfig)
	mux.HandleFunc("POST /v1/machines", s.addMachine)
	mux.HandleFunc("PUT /v1/machines/{name}", s.putMachine)
	mux.HandleFunc("DELETE /v1/machines/{name}", s.deleteMachine)
	mux.HandleFunc("POST /v1/machines/restore-defaults", s.restoreDefaults)
	mux.HandleFunc("GET /v1/machines/{name}/nodes/{node}/models", s.listModels)
	mux.HandleFunc("GET /v1/policy/targets", s.targets)
	mux.HandleFunc("POST /v1/names/check", s.checkName)
	mux.HandleFunc("POST /v1/plans/copy", s.planCopy)
	mux.HandleFunc("POST /v1/plans/rename", s.planRename)
	mux.HandleFunc("POST /v1/jobs", s.startJob)
	mux.HandleFunc("GET /v1/jobs/{id}", s.getJob)
	mux.HandleFunc("POST /v1/jobs/{id}/cancel", s.cancelJob)
	mux.HandleFunc("POST /v1/export/excel", s.exportExcel)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// DNS rebinding guard: only the literal loopback address is served.
		if r.Host != listenAddr {
			fail(w, http.StatusForbidden, "bad host")
			return
		}
		got := r.Header.Get("X-Afvi-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			fail(w, http.StatusUnauthorized, "bad token")
			return
		}
		lang := i18n.Parse(r.Header.Get("X-Afvi-Lang"))
		mux.ServeHTTP(w, r.WithContext(i18n.With(r.Context(), lang)))
	})
}

// Serve listens on addr (use 127.0.0.1:0 for a free port) and returns the
// listener address plus a function that blocks until the server stops.
func (s *Server) Serve(ctx context.Context, addr string) (net.Addr, func() error, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	if !ln.Addr().(*net.TCPAddr).IP.IsLoopback() {
		ln.Close()
		return nil, nil, errors.New("only loopback addresses are allowed")
	}
	srv := &http.Server{Handler: s.Handler(ln.Addr().String()), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(c)
	}()
	return ln.Addr(), func() error {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}, nil
}

// ---- helpers --------------------------------------------------------------

func reply(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	reply(w, code, map[string]string{"error": msg})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "bad request: "+err.Error())
		return false
	}
	return true
}

// ---- handlers -------------------------------------------------------------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	reply(w, 200, map[string]any{
		"version":           s.Version,
		"os":                runtime.GOOS,
		"allowNetworkWrite": s.settings().AllowNetworkWrite,
		"configPath":        s.cfg.Path(),
	})
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	c := s.cfg.Get()
	reply(w, 200, map[string]any{
		"machines": c.Machines,
		"settings": map[string]any{
			"smbUser":           c.Settings.SMBUser,
			"allowNetworkWrite": s.settings().AllowNetworkWrite,
		},
		"kinds": machine.Kinds,
	})
}

func (s *Server) addMachine(w http.ResponseWriter, r *http.Request) {
	tr := i18n.FromContext(r.Context())
	var m machine.Machine
	if !decode(w, r, &m) {
		return
	}
	err := s.cfg.Update(tr, func(c *machine.Config) error {
		if _, ok := c.Find(m.Name); ok {
			return errors.New(tr("m.exists", m.Name))
		}
		c.Machines = append(c.Machines, m)
		return nil
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	reply(w, 201, m)
}

func (s *Server) putMachine(w http.ResponseWriter, r *http.Request) {
	tr := i18n.FromContext(r.Context())
	name := r.PathValue("name")
	var m machine.Machine
	if !decode(w, r, &m) {
		return
	}
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
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	reply(w, 200, m)
}

func (s *Server) deleteMachine(w http.ResponseWriter, r *http.Request) {
	tr := i18n.FromContext(r.Context())
	name := r.PathValue("name")
	err := s.cfg.Update(tr, func(c *machine.Config) error {
		for i := range c.Machines {
			if strings.EqualFold(c.Machines[i].Name, name) {
				c.Machines = append(c.Machines[:i], c.Machines[i+1:]...)
				return nil
			}
		}
		return errors.New(tr("m.not_found", name))
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}

// restoreDefaults puts the built-in AFVI 13 back when it is missing.
func (s *Server) restoreDefaults(w http.ResponseWriter, r *http.Request) {
	tr := i18n.FromContext(r.Context())
	err := s.cfg.Update(tr, func(c *machine.Config) error {
		d := machine.DefaultMachine()
		if _, ok := c.Find(d.Name); !ok {
			c.Machines = append(c.Machines, d)
		}
		return nil
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	reply(w, 200, map[string]bool{"ok": true})
}

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

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	ref := policy.Ref{Machine: r.PathValue("name"), Node: machine.NodeKind(strings.ToUpper(r.PathValue("node")))}
	st, err := s.Store(r.Context(), ref)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	bundles, err := model.Scan(st)
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
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
	reply(w, 200, map[string]any{
		"models":     rows,
		"readOnly":   st.ReadOnly(),
		"repository": st.Repo.Describe(),
		"inventory":  st.Inv.Describe(),
	})
}

func (s *Server) targets(w http.ResponseWriter, r *http.Request) {
	tr := i18n.FromContext(r.Context())
	src := policy.Ref{Machine: r.URL.Query().Get("machine"), Node: machine.NodeKind(strings.ToUpper(r.URL.Query().Get("node")))}
	c := s.cfg.Get()
	if _, ok := c.Find(src.Machine); !ok {
		fail(w, http.StatusBadRequest, tr("m.not_found", src.Machine))
		return
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
	reply(w, 200, map[string]any{"single": policy.Targets(src, names), "group": group})
}

func (s *Server) checkName(w http.ResponseWriter, r *http.Request) {
	tr := i18n.FromContext(r.Context())
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	out := map[string]any{"ok": true, "class": model.Classify(req.Name)}
	if err := model.ValidateName(tr, req.Name); err != nil {
		out["ok"] = false
		out["error"] = err.Error()
	}
	reply(w, 200, out)
}

func (s *Server) planCopy(w http.ResponseWriter, r *http.Request) {
	var req ops.CopyRequest
	if !decode(w, r, &req) {
		return
	}
	pl, err := ops.PlanCopy(r.Context(), s, req)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	s.jobs.Add(pl)
	reply(w, 200, pl)
}

func (s *Server) planRename(w http.ResponseWriter, r *http.Request) {
	var req ops.RenameRequest
	if !decode(w, r, &req) {
		return
	}
	pl, err := ops.PlanRename(r.Context(), s, req)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	s.jobs.Add(pl)
	reply(w, 200, pl)
}

func (s *Server) startJob(w http.ResponseWriter, r *http.Request) {
	tr := i18n.FromContext(r.Context())
	var req struct {
		PlanID string `json:"planId"`
	}
	if !decode(w, r, &req) {
		return
	}
	jv, err := s.jobs.Start(req.PlanID)
	switch {
	case errors.Is(err, ops.ErrBusy):
		fail(w, http.StatusConflict, tr("j.busy"))
	case errors.Is(err, ops.ErrNoPlan):
		fail(w, http.StatusNotFound, tr("j.no_plan"))
	case err != nil:
		fail(w, http.StatusBadRequest, err.Error())
	default:
		reply(w, 202, jv)
	}
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	jv, ok := s.jobs.Get(r.PathValue("id"))
	if !ok {
		fail(w, http.StatusNotFound, "no such job")
		return
	}
	reply(w, 200, jv)
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	reply(w, 200, map[string]bool{"canceled": s.jobs.Cancel(r.PathValue("id"))})
}

// exportExcel is the entry point of "Export Data To Excel". The analysis and
// the workbook are not implemented yet.
func (s *Server) exportExcel(w http.ResponseWriter, r *http.Request) {
	tr := i18n.FromContext(r.Context())
	fail(w, http.StatusNotImplemented, tr("s.excel_na"))
}
