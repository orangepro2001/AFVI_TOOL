// Package server is the HTTP/JSON transport of the service layer, for a GUI that runs
// as a separate process. It listens on loopback only and every request must carry the
// per-launch token the core printed on stdout.
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
	"time"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/ops"
	"afvi/tools/core/internal/service"
)

// Server serves a service.Service over HTTP.
type Server struct {
	Version string
	svc     *service.Service
	token   string
}

// New creates a server. forceWrite lifts the read-only lock on network paths.
func New(version string, cfg *machine.Store, forceWrite bool) *Server {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return &Server{Version: version, svc: service.New(version, cfg, forceWrite), token: hex.EncodeToString(b)}
}

// Token is the secret the client must send in X-Afvi-Token.
func (s *Server) Token() string { return s.token }

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

// ---- handlers: decode, call the service, encode ---------------------------------

func (s *Server) result(w http.ResponseWriter, ok int, v any, err error) {
	if err != nil {
		fail(w, service.Status(err), err.Error())
		return
	}
	reply(w, ok, v)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	reply(w, 200, s.svc.Health(r.Context()))
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	reply(w, 200, s.svc.GetConfig(r.Context()))
}

func (s *Server) addMachine(w http.ResponseWriter, r *http.Request) {
	var m machine.Machine
	if !decode(w, r, &m) {
		return
	}
	out, err := s.svc.AddMachine(r.Context(), m)
	s.result(w, 201, out, err)
}

func (s *Server) putMachine(w http.ResponseWriter, r *http.Request) {
	var m machine.Machine
	if !decode(w, r, &m) {
		return
	}
	out, err := s.svc.UpdateMachine(r.Context(), r.PathValue("name"), m)
	s.result(w, 200, out, err)
}

func (s *Server) deleteMachine(w http.ResponseWriter, r *http.Request) {
	s.result(w, 200, map[string]bool{"ok": true}, s.svc.DeleteMachine(r.Context(), r.PathValue("name")))
}

func (s *Server) restoreDefaults(w http.ResponseWriter, r *http.Request) {
	s.result(w, 200, map[string]bool{"ok": true}, s.svc.RestoreDefaults(r.Context()))
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.ListModels(r.Context(), r.PathValue("name"), r.PathValue("node"))
	s.result(w, 200, out, err)
}

func (s *Server) targets(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.Targets(r.Context(), r.URL.Query().Get("machine"), r.URL.Query().Get("node"))
	s.result(w, 200, out, err)
}

func (s *Server) checkName(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	reply(w, 200, s.svc.CheckName(r.Context(), req.Name))
}

func (s *Server) planCopy(w http.ResponseWriter, r *http.Request) {
	var req ops.CopyRequest
	if !decode(w, r, &req) {
		return
	}
	out, err := s.svc.PlanCopy(r.Context(), req)
	s.result(w, 200, out, err)
}

func (s *Server) planRename(w http.ResponseWriter, r *http.Request) {
	var req ops.RenameRequest
	if !decode(w, r, &req) {
		return
	}
	out, err := s.svc.PlanRename(r.Context(), req)
	s.result(w, 200, out, err)
}

func (s *Server) startJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlanID string `json:"planId"`
	}
	if !decode(w, r, &req) {
		return
	}
	out, err := s.svc.StartJob(r.Context(), req.PlanID)
	s.result(w, 202, out, err)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	out, err := s.svc.GetJob(r.Context(), r.PathValue("id"))
	s.result(w, 200, out, err)
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	reply(w, 200, map[string]bool{"canceled": s.svc.CancelJob(r.Context(), r.PathValue("id"))})
}

func (s *Server) exportExcel(w http.ResponseWriter, r *http.Request) {
	s.result(w, 200, nil, s.svc.ExportExcel(r.Context()))
}
