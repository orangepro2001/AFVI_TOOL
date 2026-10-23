package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/model"
	"afvi/tools/core/internal/server"
	"afvi/tools/core/internal/testkit"
	"afvi/tools/core/internal/vfs"
)

type fixture struct {
	t     *testing.T
	base  string
	token string
	roots map[string]model.Store
}

func localMachine(t *testing.T, root, name string) (machine.Machine, map[string]model.Store) {
	stores := map[string]model.Store{}
	node := func(k string) machine.Node {
		dir := filepath.Join(root, name, k)
		stores[k] = testkit.NewStore(t, dir)
		return machine.Node{Host: "local", RepositoryPath: filepath.Join(dir, "PxRepository"), InventoryPath: filepath.Join(dir, "PxInventory")}
	}
	return machine.Machine{Name: name, FM1: node("FM1"), FM2: node("FM2"), BM: node("BM")}, stores
}

func newFixture(t *testing.T) *fixture {
	root := t.TempDir()
	cfg, _, err := machine.Open(filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	fx := &fixture{t: t, roots: map[string]model.Store{}}
	for _, n := range []string{"LOCAL A", "LOCAL B"} {
		m, st := localMachine(t, root, n)
		for k, s := range st {
			fx.roots[n+"/"+k] = s
		}
		if err := cfg.Update(i18n.For(i18n.En), func(c *machine.Config) error { c.Machines = append(c.Machines, m); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	srv := server.New("test", cfg, false)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	addr, wait, err := srv.Serve(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go wait()
	fx.base = "http://" + addr.String()
	fx.token = srv.Token()
	return fx
}

func (fx *fixture) do(method, path string, body any, token string) (int, map[string]any) {
	fx.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, fx.base+path, rd)
	if token != "" {
		req.Header.Set("X-Afvi-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fx.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (fx *fixture) call(method, path string, body any) (int, map[string]any) {
	return fx.do(method, path, body, fx.token)
}

func TestAuth(t *testing.T) {
	fx := newFixture(t)
	if code, _ := fx.do("GET", "/v1/health", nil, ""); code != 401 {
		t.Errorf("no token: %d", code)
	}
	if code, _ := fx.do("GET", "/v1/health", nil, "wrong"); code != 401 {
		t.Errorf("wrong token: %d", code)
	}
	req, _ := http.NewRequest("GET", fx.base+"/v1/health", nil)
	req.Host = "evil.example:80"
	req.Header.Set("X-Afvi-Token", fx.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("foreign Host header: %d", resp.StatusCode)
	}
	if code, h := fx.call("GET", "/v1/health", nil); code != 200 || h["allowNetworkWrite"] != false {
		t.Errorf("health: %d %v", code, h)
	}
}

func TestMachinesAndModelList(t *testing.T) {
	fx := newFixture(t)
	code, cfg := fx.call("GET", "/v1/config", nil)
	if code != 200 || len(cfg["machines"].([]any)) != 3 { // AFVI 13 + two local
		t.Fatalf("config: %d %v", code, cfg)
	}
	testkit.AddModel(t, fx.roots["LOCAL A/FM1"], "24fbo060-19", "x")
	testkit.Write(t, fx.roots["LOCAL A/FM1"].Repo, "danny/a.txt", "x")
	code, ml := fx.call("GET", "/v1/machines/LOCAL%20A/nodes/fm1/models", nil)
	if code != 200 {
		t.Fatalf("list: %d %v", code, ml)
	}
	rows := ml["models"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["name"] != "24fbo060-19" || rows[0].(map[string]any)["inspect"] != float64(1) {
		t.Errorf("rows: %v", rows)
	}
	if ml["readOnly"] != false {
		t.Error("local path must be writable")
	}
	// add / update / delete a machine
	m := machine.DefaultMachine()
	m.Name = "AFVI 14"
	if code, _ := fx.call("POST", "/v1/machines", m); code != 201 {
		t.Errorf("add: %d", code)
	}
	if code, _ := fx.call("POST", "/v1/machines", m); code != 400 {
		t.Errorf("duplicate add: %d", code)
	}
	m.FM1.Host = "10.1.1.1"
	if code, _ := fx.call("PUT", "/v1/machines/AFVI%2014", m); code != 200 {
		t.Errorf("update: %d", code)
	}
	if code, _ := fx.call("DELETE", "/v1/machines/AFVI%2014", nil); code != 200 {
		t.Errorf("delete: %d", code)
	}
	if code, _ := fx.call("DELETE", "/v1/machines/AFVI%2013", nil); code != 200 {
		t.Errorf("delete default: %d", code)
	}
	if code, _ := fx.call("POST", "/v1/machines/restore-defaults", nil); code != 200 {
		t.Errorf("restore: %d", code)
	}
	_, cfg = fx.call("GET", "/v1/config", nil)
	if len(cfg["machines"].([]any)) != 3 {
		t.Errorf("after restore: %v", cfg["machines"])
	}
}

func (fx *fixture) wait(jobID string) map[string]any {
	fx.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, j := fx.call("GET", "/v1/jobs/"+jobID, nil)
		if j["state"] != "running" {
			return j
		}
		if time.Now().After(deadline) {
			fx.t.Fatal("job timeout")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCopyOverHTTP(t *testing.T) {
	fx := newFixture(t)
	testkit.AddModel(t, fx.roots["LOCAL A/FM1"], "m1", "NEW")
	testkit.AddModel(t, fx.roots["LOCAL A/FM2"], "M1", "OLD")

	_, tg := fx.call("GET", "/v1/policy/targets?machine=LOCAL%20A&node=FM1", nil)
	if len(tg["single"].([]any)) != 2 || len(tg["group"].([]any)) != 2 { // FM1,FM2 of A; groups: B and AFVI 13
		t.Errorf("targets: %v", tg)
	}
	code, pl := fx.call("POST", "/v1/plans/copy", map[string]any{
		"mode": "single", "model": "m1",
		"source": map[string]string{"machine": "LOCAL A", "node": "FM1"},
		"target": map[string]string{"machine": "LOCAL A", "node": "FM2"}})
	if code != 200 || len(pl["blockers"].([]any)) != 0 {
		t.Fatalf("plan: %d %v", code, pl)
	}
	code, job := fx.call("POST", "/v1/jobs", map[string]string{"planId": pl["id"].(string)})
	if code != 202 {
		t.Fatalf("start: %d %v", code, job)
	}
	if done := fx.wait(job["id"].(string)); done["state"] != "done" {
		t.Fatalf("job: %v", done)
	}
	if got := testkit.Read(t, fx.roots["LOCAL A/FM2"].Repo, "m1/2.0/Line/Top/Top_handler.txt"); got != "handler NEW" {
		t.Errorf("copied content: %q", got)
	}
	if code, _ := fx.call("POST", "/v1/jobs", map[string]string{"planId": pl["id"].(string)}); code != 404 {
		t.Errorf("a plan must start once: %d", code)
	}
	// forbidden combination comes back as 400 with the reason
	code, bad := fx.call("POST", "/v1/plans/copy", map[string]any{
		"mode": "single", "model": "m1",
		"source": map[string]string{"machine": "LOCAL A", "node": "FM1"},
		"target": map[string]string{"machine": "LOCAL A", "node": "BM"}})
	if code != 400 || bad["error"] == "" {
		t.Errorf("FM -> BM: %d %v", code, bad)
	}
	// rename + name check + excel entry
	code, pl = fx.call("POST", "/v1/plans/rename", map[string]any{"mode": "group", "model": "m1", "newName": "m9",
		"ref": map[string]string{"machine": "LOCAL A", "node": "FM1"}})
	if code != 200 || len(pl["units"].([]any)) != 2 {
		t.Fatalf("rename plan: %d %v", code, pl)
	}
	_, job = fx.call("POST", "/v1/jobs", map[string]string{"planId": pl["id"].(string)})
	if done := fx.wait(job["id"].(string)); done["state"] != "done" {
		t.Fatalf("rename job: %v", done)
	}
	if !testkit.Exists(fx.roots["LOCAL A/FM2"].Repo, "m9") || testkit.Exists(fx.roots["LOCAL A/FM1"].Repo, "m1") {
		t.Error("group rename did not happen")
	}
	if _, r := fx.call("POST", "/v1/names/check", map[string]string{"name": "a/b"}); r["ok"] != false {
		t.Errorf("name check: %v", r)
	}
	if code, _ := fx.call("POST", "/v1/export/excel", map[string]any{}); code != 501 {
		t.Errorf("excel entry: %d", code)
	}
}

func TestNetworkTargetIsReadOnlyByDefault(t *testing.T) {
	fx := newFixture(t)
	// AFVI 13 lives on UNC paths: on this OS they are opened read-only.
	testkit.AddModel(t, fx.roots["LOCAL A/FM1"], "m1", "NEW")
	code, pl := fx.call("POST", "/v1/plans/copy", map[string]any{
		"mode": "group", "model": "m1",
		"source": map[string]string{"machine": "LOCAL A", "node": "FM1"},
		"target": map[string]string{"machine": "AFVI 13", "node": "FM1"}})
	if code != 200 {
		t.Skipf("network not reachable here: %d %v", code, pl)
	}
	if len(pl["blockers"].([]any)) == 0 {
		t.Fatalf("writing to the network must be blocked: %v", pl)
	}
	code, r := fx.call("POST", "/v1/jobs", map[string]string{"planId": pl["id"].(string)})
	if code != 400 {
		t.Errorf("blocked plan started: %d %v", code, r)
	}
	var _ = vfs.ErrReadOnly
}

func TestLanguageHeader(t *testing.T) {
	fx := newFixture(t)
	testkit.AddModel(t, fx.roots["LOCAL A/FM1"], "m1", "x")
	body, _ := json.Marshal(map[string]any{"mode": "single", "model": "m1",
		"source": map[string]string{"machine": "LOCAL A", "node": "FM1"},
		"target": map[string]string{"machine": "LOCAL A", "node": "BM"}})
	want := map[string]string{
		"":        "FM과 BM 사이에는 복사할 수 없습니다", // no header: Korean
		"ko":      "FM과 BM 사이에는 복사할 수 없습니다",
		"zh-Hans": "FM 与 BM 之间不允许复制",
		"zh-Hant": "FM 與 BM 之間不允許複製",
		"en":      "Copying between FM and BM is not allowed",
		"en-US":   "Copying between FM and BM is not allowed",
	}
	for lang, text := range want {
		req, _ := http.NewRequest("POST", fx.base+"/v1/plans/copy", bytes.NewReader(body))
		req.Header.Set("X-Afvi-Token", fx.token)
		if lang != "" {
			req.Header.Set("X-Afvi-Lang", lang)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]string
		json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if resp.StatusCode != 400 || out["error"] != text {
			t.Errorf("lang %q: %d %q, want %q", lang, resp.StatusCode, out["error"], text)
		}
	}
}
