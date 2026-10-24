package main

import (
	"context"
	"fmt"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/service"

	"github.com/energye/golcl/lcl"
	"github.com/energye/golcl/lcl/types"
)

// App is what every window shares: the service (in process, no HTTP), the language and the
// plumbing to get back onto the UI thread.
type App struct {
	svc *service.Service
	loc *Loc

	// autoConfirm answers every plan and message dialog by itself (--selftest).
	autoConfirm    bool
	fontPt         int32
	lastMessage    string
	lastPlanDanger bool

	deferred []func()
	kick     *lcl.TTimer
}

func (a *App) ctx() context.Context {
	return i18n.With(context.Background(), i18n.Parse(a.loc.Tag()))
}

func (a *App) T(key string, args ...any) string { return a.loc.T(key, args...) }

// initDefer creates the timer that runs deferred calls on the UI thread, outside the
// ThreadSync that queued them (a modal dialog must not block inside ThreadSync).
func (a *App) initDefer(owner lcl.IComponent) {
	a.kick = lcl.NewTimer(owner)
	a.kick.SetEnabled(false)
	a.kick.SetInterval(1)
	a.kick.SetOnTimer(func(lcl.IObject) {
		a.kick.SetEnabled(false)
		for len(a.deferred) > 0 {
			fn := a.deferred[0]
			a.deferred = a.deferred[1:]
			fn()
		}
	})
}

// inUI runs fn on the UI thread; call it from a goroutine.
func (a *App) inUI(fn func()) {
	lcl.ThreadSync(func() {
		a.deferred = append(a.deferred, fn)
		a.kick.SetEnabled(true)
	})
}

// quick runs fn on the UI thread inside ThreadSync (no dialogs in fn).
func quick(fn func()) { lcl.ThreadSync(fn) }

func (a *App) showMessage(text string, isError bool) {
	if a.autoConfirm {
		a.lastMessage = text
		fmt.Println("MESSAGE:", text)
		return
	}
	var kind types.TMsgDlgType = types.MtInformation
	if isError {
		kind = types.MtError
	}
	lcl.MessageDlg(text, kind, types.MbOK)
}

func (a *App) confirm(text string) bool {
	if a.autoConfirm {
		return true
	}
	return lcl.MessageDlg(text, types.MtConfirmation, types.MbYes, types.MbNo) == types.IdYes
}

func human(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

// label creates a plain caption.
func newLabel(parent lcl.IWinControl, text string, x, y int32) *lcl.TLabel {
	l := lcl.NewLabel(parent)
	l.SetParent(parent)
	l.SetCaption(text)
	l.SetLeft(x)
	l.SetTop(y)
	return l
}

func newEdit(parent lcl.IWinControl, x, y, w int32) *lcl.TEdit {
	e := lcl.NewEdit(parent)
	e.SetParent(parent)
	e.SetBounds(x, y, w, e.Height())
	return e
}

func newCombo(parent lcl.IWinControl, x, y, w int32) *lcl.TComboBox {
	c := lcl.NewComboBox(parent)
	c.SetParent(parent)
	c.SetStyle(types.CsDropDownList)
	c.SetBounds(x, y, w, c.Height())
	return c
}

func wrapLabel(parent lcl.IWinControl, x, y, w, h int32) *lcl.TLabel {
	l := lcl.NewLabel(parent)
	l.SetParent(parent)
	l.SetAutoSize(false)
	l.SetWordWrap(true)
	l.SetBounds(x, y, w, h)
	return l
}
