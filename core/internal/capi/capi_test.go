package capi_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"afvi/tools/core/internal/capi"
)

type reply struct {
	OK     bool            `json:"ok"`
	Error  string          `json:"error"`
	Result json.RawMessage `json:"result"`
}

func call(t *testing.T, method, lang, args string) reply {
	t.Helper()
	var r reply
	if err := json.Unmarshal([]byte(capi.Call(method, lang, args)), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestJourneyThroughTheCABIShape(t *testing.T) {
	if r := call(t, "Health", "en", ""); r.OK || !strings.Contains(r.Error, "not initialised") {
		t.Fatalf("before init: %+v", r)
	}
	if err := capi.Init("", t.TempDir(), false); err != nil {
		t.Fatal(err)
	}
	if r := call(t, "SelftestEnabled", "en", ""); !r.OK || string(r.Result) != "true" {
		t.Fatalf("selftest: %+v", r)
	}
	r := call(t, "ListModels", "ko", `{"machine":"T1","node":"FM1"}`)
	var list struct{ Models []struct{ Name string } }
	if json.Unmarshal(r.Result, &list); !r.OK || len(list.Models) != 3 {
		t.Fatalf("list: %+v", r)
	}
	r = call(t, "PlanCopy", "en", `{"request":{"mode":"single","model":"m1","source":{"machine":"T1","node":"FM1"},"target":{"machine":"T1","node":"FM2"}}}`)
	var plan struct{ ID string }
	if json.Unmarshal(r.Result, &plan); !r.OK || plan.ID == "" {
		t.Fatalf("plan: %+v", r)
	}
	r = call(t, "StartJob", "en", `{"planId":"`+plan.ID+`"}`)
	var job struct{ ID, State string }
	json.Unmarshal(r.Result, &job)
	for i := 0; i < 400 && job.State == "running"; i++ {
		time.Sleep(10 * time.Millisecond)
		r = call(t, "GetJob", "en", `{"id":"`+job.ID+`"}`)
		json.Unmarshal(r.Result, &job)
	}
	if job.State != "done" {
		t.Fatalf("job: %+v", job)
	}
	if r := call(t, "SelftestVerify", "en", `{"step":"copy"}`); string(r.Result) != `""` {
		t.Fatalf("verify: %s", r.Result)
	}
}

func TestErrorsAreJSONAndNeverPanic(t *testing.T) {
	_ = capi.Init("", t.TempDir(), false)
	for _, c := range []struct{ m, l, a, want string }{
		{"Nope", "en", "", "unknown method"},
		{"ListModels", "en", "{bad", "bad arguments"},
		{"PlanCopy", "en", `{}`, "missing argument"},
		{"PlanCopy", "zh-Hant", `{"request":{"mode":"single","model":"m1","source":{"machine":"T1","node":"FM1"},"target":{"machine":"T1","node":"BM"}}}`, "FM 與 BM"},
		{"ExportExcel", "ko", "", "구현되지"},
	} {
		if r := call(t, c.m, c.l, c.a); r.OK || !strings.Contains(r.Error, c.want) {
			t.Errorf("%s: %+v, want error containing %q", c.m, r, c.want)
		}
	}
}
