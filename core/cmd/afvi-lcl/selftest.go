package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"afvi/tools/core/internal/wailsapp"

	"github.com/energye/golcl/lcl"
)

// selftestRun drives the real window through its own handlers, one step per timer tick, like a person would:
// choose, click, wait for the dialogs (which answer themselves in self-test mode) and then ask Go to check the files.
type selftestRun struct {
	wapp  *wailsapp.App
	code  int
	steps []stStep
	cur   int
	start time.Time
	log   []string
}

type stStep struct {
	name string
	// do starts the step (called once); check is polled until it returns true or the step times out.
	do    func(f *TMainForm)
	check func(f *TMainForm) bool
}

func (s *selftestRun) exitCode() int { return s.code }

func (s *selftestRun) fail(f *TMainForm, msg string) {
	fmt.Fprintln(os.Stderr, "SELFTEST FAIL: "+msg+" [done: "+strings.Join(s.log, ",")+"]")
	s.code = 1
	f.Close()
}

func selectRow(f *TMainForm, name string) bool {
	items := f.lv.Items()
	for i := int32(0); i < items.Count(); i++ {
		if it := items.Item(i); strings.EqualFold(it.Caption(), name) {
			f.lv.SetSelected(it)
			f.selected = it.Caption()
			f.updateEnabled()
			return true
		}
	}
	return false
}

func comboIndex(c *lcl.TComboBox, text string) int32 {
	for i := int32(0); i < c.Items().Count(); i++ {
		if c.Items().Strings(i) == text {
			return i
		}
	}
	return -1
}

func (s *selftestRun) buildSteps() {
	idle := func(f *TMainForm) bool { return !f.busy }
	s.steps = []stStep{
		{"list", func(f *TMainForm) {
			f.app.loc.Set("en")
			f.applyTexts()
			f.cbMachine.SetItemIndex(comboIndex(f.cbMachine, "T1"))
			f.cbNode.SetItemIndex(0)
			f.onSourceChanged()
		}, func(f *TMainForm) bool { return len(f.view) == 3 && f.curMachine() == "T1" }},
		{"search", func(f *TMainForm) { f.edSearch.SetText("M99"); f.applyFilter() },
			func(f *TMainForm) bool { return len(f.view) == 1 && f.view[0].Name == "7M99-535" }},
		{"search-clear", func(f *TMainForm) { f.edSearch.SetText(""); f.applyFilter() },
			func(f *TMainForm) bool { return len(f.view) == 3 }},
		{"languages", func(f *TMainForm) {
			seen := map[string]bool{}
			for _, tag := range langTags {
				f.setLanguage(tag)
				want := f.app.loc.packs[tag]["tab_copy"]
				got := f.tabCaption(0)
				if os.Getenv("AFVI_LCL_DEBUG") != "" {
					fmt.Fprintf(os.Stderr, "lang %s: tab=%q\n", tag, got)
				}
				if got != want {
					s.fail(f, fmt.Sprintf("language %s: tab shows %q, want %q", tag, got, want))
					return
				}
				seen[got] = true
			}
			if len(seen) < 4 {
				s.fail(f, "languages look the same")
			}
			f.setLanguage("en")
		}, func(f *TMainForm) bool { return true }},
		{"no-bm-target", func(f *TMainForm) {
			if comboIndex(f.cbSingle, "T1 / BM") >= 0 {
				s.fail(f, "BM offered as a target of FM1")
			}
		}, func(f *TMainForm) bool { return true }},
		{"copy", func(f *TMainForm) {
			if !selectRow(f, "m1") {
				s.fail(f, "m1 not listed")
				return
			}
			f.cbSingle.SetItemIndex(comboIndex(f.cbSingle, "T1 / FM2"))
			f.rbSingle.SetChecked(true)
			f.updateEnabled()
			f.btnCopy.Click()
		}, func(f *TMainForm) bool {
			if !idle(f) {
				return false
			}
			if !f.app.lastPlanDanger {
				s.fail(f, "the deletion of the old M1 was not shown as a danger step")
			} else if v := s.wapp.SelftestVerify("copy"); v != "" {
				s.fail(f, "copy: "+v)
			}
			return true
		}},
		{"rename-group", func(f *TMainForm) {
			f.cbNode.SetItemIndex(0)
			f.onSourceChanged()
		}, func(f *TMainForm) bool { return selectRow(f, "m2") }},
		{"rename-run", func(f *TMainForm) {
			f.chkRenameGroup.SetChecked(true)
			f.edRenameName.SetText("r2")
			f.btnRename.Click()
		}, func(f *TMainForm) bool {
			if !idle(f) {
				return false
			}
			if v := s.wapp.SelftestVerify("rename"); v != "" {
				s.fail(f, "rename: "+v)
			}
			return true
		}},
		{"refusal", func(f *TMainForm) {
			f.chkRenameGroup.SetChecked(false)
			f.edRenameName.SetText("bad/name")
			f.app.lastMessage = ""
		}, func(f *TMainForm) bool { return selectRow(f, "r2") }},
		{"refusal-run", func(f *TMainForm) { f.btnRename.Click() }, func(f *TMainForm) bool {
			if !idle(f) {
				return false
			}
			if !strings.Contains(f.app.lastMessage, "illegal character") {
				s.fail(f, "refusal text: "+f.app.lastMessage)
			}
			return true
		}},
		{"excel", func(f *TMainForm) { f.app.lastMessage = ""; f.exportExcel() }, func(f *TMainForm) bool {
			if !strings.Contains(f.app.lastMessage, "not implemented") {
				s.fail(f, "excel entry text: "+f.app.lastMessage)
			}
			return true
		}},
	}
}

// run starts the state machine on the main form's timer.
func (s *selftestRun) run(f *TMainForm) {
	s.buildSteps()
	s.start = time.Now()
	started := false
	t := lcl.NewTimer(f)
	t.SetInterval(80)
	t.SetOnTimer(func(lcl.IObject) {
		if s.cur >= len(s.steps) {
			t.SetEnabled(false)
			if s.code == 0 {
				fmt.Println("SELFTEST OK " + strings.Join(s.log, ","))
			}
			f.Close()
			return
		}
		st := s.steps[s.cur]
		if !started {
			started = true
			s.start = time.Now()
			st.do(f)
			if s.code != 0 {
				t.SetEnabled(false)
			}
			return
		}
		if s.code != 0 {
			return
		}
		if st.check(f) {
			if s.code == 0 {
				s.log = append(s.log, st.name)
			}
			s.cur++
			started = false
			return
		}
		if time.Since(s.start) > 20*time.Second {
			t.SetEnabled(false)
			s.fail(f, "timeout in step "+st.name)
		}
	})
}
