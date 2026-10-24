package main

import (
	"fmt"

	"afvi/tools/core/internal/ops"

	"github.com/energye/golcl/lcl"
	"github.com/energye/golcl/lcl/types"
)

// Dialogs are real modal forms built in code. In self-test mode (app.autoConfirm) they answer themselves
// after a short moment, so the whole chain of windows is exercised without a person.

func newDialog(app *App, owner *lcl.TForm, title string, w, h int32) *lcl.TForm {
	d := lcl.NewForm(owner)
	d.SetCaption(title)
	d.SetPosition(types.PoOwnerFormCenter)
	d.SetWidth(w)
	d.SetHeight(h)
	d.SetBorderStyle(types.BsDialog)
	if curSkin != nil {
		d.SetColor(curSkin.Face())
	}
	return d
}

// stepList is a list box that paints its lines in the colour that says what they are.
type line struct {
	text string
	kind int // 0 normal, 1 heading, 2 danger, 3 warning, 4 blocker
}

func newLineList(parent lcl.IWinControl, lines *[]line) *lcl.TListBox {
	lb := lcl.NewListBox(parent)
	lb.SetParent(parent)
	lb.SetStyle(types.LbOwnerDrawFixed)
	lb.SetItemHeight(18)
	lb.SetOnDrawItem(func(_ lcl.IWinControl, i int32, r types.TRect, st types.TOwnerDrawState) {
		c := lb.Canvas()
		bg := types.TColor(0xFFFFFF)
		if curSkin != nil {
			bg = curSkin.Field()
		}
		fill(c, r.Left, r.Top, r.Right, r.Bottom, bg)
		if int(i) >= len(*lines) {
			return
		}
		ln := (*lines)[i]
		col := rgb(0, 0, 0)
		switch ln.kind {
		case 2, 4:
			col = rgb(0xC0, 0, 0)
		case 3:
			col = rgb(0x80, 0x60, 0)
		}
		c.Brush().SetStyle(types.BsClear)
		c.Font().SetColor(col)
		c.TextOut(r.Left+4, r.Top+1, ln.text)
		if ln.kind == 1 { // bold look: drawn twice, 1 px apart
			c.TextOut(r.Left+5, r.Top+1, ln.text)
		}
		c.Brush().SetStyle(types.BsSolid)
	})
	return lb
}

// showPlan returns true when the user ran the plan.
func showPlan(app *App, owner *lcl.TForm, p *ops.Plan) bool {
	d := newDialog(app, owner, app.T("plan_title"), 760, 520)
	title := newLabel(d, p.Title, 12, 10)
	title.Font().SetStyle(types.NewSet(types.FsBold))

	var lines []line
	for _, b := range p.Blockers {
		lines = append(lines, line{app.T("plan_blocker", b), 4})
	}
	for _, w := range p.Warnings {
		lines = append(lines, line{app.T("plan_warning", w), 3})
	}
	danger := false
	for _, u := range p.Units {
		h := "[" + u.Target.Machine + " / " + string(u.Target.Node) + "]  " + u.Model
		if u.NewName != u.Model {
			h += " → " + u.NewName
		}
		lines = append(lines, line{h, 1})
		for _, s := range u.Steps {
			k := 0
			if s.Danger {
				k, danger = 2, true
			}
			lines = append(lines, line{"    " + s.Text, k})
		}
	}
	app.lastPlanDanger = danger
	lb := newLineList(d, &lines)
	lb.SetBounds(12, 38, 730, 392)
	for _, l := range lines {
		lb.Items().Add(l.text)
	}

	sum := app.T("plan_summary_rename", len(p.Units))
	if p.Kind == "copy" {
		sum = app.T("plan_summary_copy", p.Files, human(p.Bytes))
	}
	newLabel(d, sum, 12, 446)
	run := newButton(d, 536, 440, 100, 28, func() { d.SetModalResult(types.MrOk) })
	run.SetCaption(app.T("btn_run"))
	run.SetEnabled(len(p.Blockers) == 0)
	run.SetDefault(true)
	cancel := newButton(d, 644, 440, 100, 28, func() { d.SetModalResult(types.MrCancel) })
	cancel.SetCaption(app.T("btn_cancel"))

	if app.autoConfirm {
		t := lcl.NewTimer(d)
		t.SetInterval(400)
		t.SetOnTimer(func(lcl.IObject) {
			t.SetEnabled(false)
			if run.enabled {
				d.SetModalResult(types.MrOk)
			} else {
				d.SetModalResult(types.MrCancel)
			}
		})
	}
	return d.ShowModal() == int32(types.MrOk)
}

// showJob shows the progress of a started job until it ends and returns its final state.
func showJob(app *App, owner *lcl.TForm, job ops.JobView) ops.JobView {
	d := newDialog(app, owner, app.T("job_running"), 660, 440)
	title := newLabel(d, job.Title, 12, 10)
	title.Font().SetStyle(types.NewSet(types.FsBold))
	phase := newLabel(d, "", 12, 34)
	bar := lcl.NewProgressBar(d)
	bar.SetParent(d)
	bar.SetBounds(12, 58, 630, 20)
	bar.SetMax(1000)
	counts := newLabel(d, "", 12, 86)
	cur := newLabel(d, "", 12, 106)
	log := lcl.NewMemo(d)
	log.SetParent(d)
	log.SetReadOnly(true)
	log.SetScrollBars(types.SsAutoVertical)
	log.SetBounds(12, 130, 630, 210)
	result := newLabel(d, "", 12, 350)
	result.Font().SetStyle(types.NewSet(types.FsBold))
	closeBtn := newButton(d, 542, 366, 100, 28, func() { d.SetModalResult(types.MrOk) })
	closeBtn.SetCaption(app.T("btn_close"))
	closeBtn.SetEnabled(false)

	final := job
	logged := 0
	finished := false
	apply := func(j ops.JobView) {
		final = j
		pr := j.Progress
		phase.SetCaption(pr.Phase)
		pos := int32(0)
		switch {
		case pr.BytesTotal > 0:
			pos = int32(1000 * pr.BytesDone / pr.BytesTotal)
		case pr.FilesTotal > 0:
			pos = int32(1000 * pr.FilesDone / pr.FilesTotal)
		}
		bar.SetPosition(pos)
		counts.SetCaption(app.T("job_counts", pr.FilesDone, pr.FilesTotal, human(pr.BytesDone), human(pr.BytesTotal), fmt.Sprintf("%.1f", pr.Seconds)))
		cur.SetCaption(pr.Current)
		for ; logged < len(pr.Log); logged++ {
			log.Lines().Add(pr.Log[logged])
		}
		if j.State != ops.StateRunning && !finished {
			finished = true
			bar.SetPosition(1000)
			if j.State == ops.StateDone {
				result.SetCaption(app.T("job_done"))
				d.SetCaption(app.T("job_done"))
			} else {
				result.SetCaption(j.Error)
				d.SetCaption(app.T("job_failed"))
			}
			closeBtn.SetEnabled(true)
			if app.autoConfirm {
				d.SetModalResult(types.MrOk)
			}
		}
	}
	apply(job)
	t := lcl.NewTimer(d)
	t.SetInterval(150)
	t.SetOnTimer(func(lcl.IObject) {
		if finished {
			return
		}
		if j, err := app.svc.GetJob(app.ctx(), job.ID); err == nil {
			apply(j)
		} else {
			apply(ops.JobView{ID: job.ID, State: ops.StateFailed, Error: err.Error(), Progress: final.Progress})
		}
	})
	d.SetOnCloseQuery(func(_ lcl.IObject, canClose *bool) { *canClose = finished }) // a running job cannot be abandoned
	d.ShowModal()
	return final
}

// showMachines is the machine list with "restore the default AFVI 13" (full add/edit is not ported in this spike).
func showMachines(app *App, owner *lcl.TForm) bool {
	d := newDialog(app, owner, app.T("mach_title"), 520, 380)
	lb := lcl.NewListBox(d)
	lb.SetParent(d)
	lb.SetBounds(12, 12, 490, 280)
	for _, m := range app.svc.GetConfig(app.ctx()).Machines {
		lb.Items().Add(m.Name + "   " + m.FM1.Host + " / " + m.FM2.Host + " / " + m.BM.Host)
	}
	changed := false
	restore := newButton(d, 12, 304, 220, 28, func() {
		if err := app.svc.RestoreDefaults(app.ctx()); err == nil {
			changed = true
			d.SetModalResult(types.MrOk)
		}
	})
	restore.SetCaption(app.T("btn_restore"))
	closeBtn := newButton(d, 402, 304, 100, 28, func() { d.SetModalResult(types.MrCancel) })
	closeBtn.SetCaption(app.T("btn_close"))
	d.ShowModal()
	return changed
}
