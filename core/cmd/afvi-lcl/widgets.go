package main

import (
	"strings"

	"github.com/energye/golcl/lcl"
	"github.com/energye/golcl/lcl/types"
)

// The controls below exist in two forms: with a skin they are a TPaintBox / TPanel drawn by the Skin,
// with -skin=native they are the stock LCL control (button, group box, status bar). The rest of the
// program only sees these wrappers.

// ---- push button ----------------------------------------------------------------------

type Btn struct {
	tb      *lcl.TButton
	pb      *lcl.TPaintBox
	caption string
	enabled bool
	down    bool
	def     bool
	onClick func()
}

func newButton(parent lcl.IWinControl, x, y, w, h int32, onClick func()) *Btn {
	b := &Btn{enabled: true, onClick: onClick}
	if curSkin == nil {
		b.tb = lcl.NewButton(parent)
		b.tb.SetParent(parent)
		b.tb.SetBounds(x, y, w, h)
		if onClick != nil {
			b.tb.SetOnClick(func(lcl.IObject) { onClick() })
		}
		return b
	}
	b.pb = lcl.NewPaintBox(parent)
	b.pb.SetParent(parent)
	b.pb.SetBounds(x, y, w, h)
	b.pb.SetOnPaint(func(lcl.IObject) {
		c := b.pb.Canvas()
		r := rect(0, 0, b.pb.Width(), b.pb.Height())
		curSkin.Button(c, r, b.caption, btnState{down: b.down, disabled: !b.enabled, def: b.def})
	})
	b.pb.SetOnMouseDown(func(_ lcl.IObject, _ types.TMouseButton, _ types.TShiftState, _, _ int32) {
		if b.enabled {
			b.down = true
			b.pb.Invalidate()
		}
	})
	b.pb.SetOnMouseUp(func(_ lcl.IObject, _ types.TMouseButton, _ types.TShiftState, x, y int32) {
		was := b.down
		b.down = false
		b.pb.Invalidate()
		if was && b.enabled && x >= 0 && y >= 0 && x < b.pb.Width() && y < b.pb.Height() && b.onClick != nil {
			b.onClick()
		}
	})
	return b
}

func (b *Btn) SetCaption(s string) {
	b.caption = s
	if b.tb != nil {
		b.tb.SetCaption(s)
	} else {
		b.pb.Invalidate()
	}
}

func (b *Btn) SetEnabled(v bool) {
	b.enabled = v
	if b.tb != nil {
		b.tb.SetEnabled(v)
	} else {
		b.pb.Invalidate()
	}
}

func (b *Btn) SetDefault(v bool) {
	b.def = v
	if b.tb != nil {
		b.tb.SetDefault(v)
	} else {
		b.pb.Invalidate()
	}
}

func (b *Btn) Control() lcl.IControl {
	if b.tb != nil {
		return b.tb
	}
	return b.pb
}

func (b *Btn) Width() int32 {
	if b.tb != nil {
		return b.tb.Width()
	}
	return b.pb.Width()
}

func (b *Btn) SetLeft(x int32) {
	if b.tb != nil {
		b.tb.SetLeft(x)
	} else {
		b.pb.SetLeft(x)
	}
}

// Click runs the button's action as if it was pressed (self-test).
func (b *Btn) Click() {
	if b.enabled && b.onClick != nil {
		b.onClick()
	}
}

// ---- group box ---------------------------------------------------------------------------

// Grp is an etched frame with a title; children go into P().
type Grp struct {
	tg    *lcl.TGroupBox
	pn    *lcl.TPanel
	inner *lcl.TPanel
	cap   string
}

func newGroup(parent lcl.IWinControl, x, y, w, h int32) *Grp {
	g := &Grp{}
	if curSkin == nil {
		g.tg = lcl.NewGroupBox(parent)
		g.tg.SetParent(parent)
		g.tg.SetBounds(x, y, w, h)
		return g
	}
	g.pn = lcl.NewPanel(parent)
	g.pn.SetParent(parent)
	g.pn.SetBevelOuter(types.BvNone)
	g.pn.SetBounds(x, y, w, h)
	g.pn.SetOnPaint(func(lcl.IObject) {
		curSkin.Group(g.pn.Canvas(), rect(0, 0, g.pn.Width(), g.pn.Height()), g.cap)
	})
	// the client panel sits inside the frame, below the title: the frame stays visible around it
	g.inner = lcl.NewPanel(g.pn)
	g.inner.SetParent(g.pn)
	g.inner.SetBevelOuter(types.BvNone)
	g.inner.SetColor(curSkin.Face())
	place := func() { g.inner.SetBounds(3, 16, g.pn.Width()-6, g.pn.Height()-19) }
	place()
	g.pn.SetOnResize(func(lcl.IObject) { place() })
	return g
}

func (g *Grp) P() lcl.IWinControl {
	if g.tg != nil {
		return g.tg
	}
	return g.inner
}

func (g *Grp) SetCaption(s string) {
	g.cap = s
	if g.tg != nil {
		g.tg.SetCaption(s)
	} else {
		g.pn.Invalidate()
	}
}

func (g *Grp) SetAlign(a types.TAlign) {
	if g.tg != nil {
		g.tg.SetAlign(a)
	} else {
		g.pn.SetAlign(a)
	}
}

func (g *Grp) SetHeight(h int32) {
	if g.tg != nil {
		g.tg.SetHeight(h)
	} else {
		g.pn.SetHeight(h)
	}
}

func (g *Grp) ClientWidth() int32 {
	if g.tg != nil {
		return g.tg.ClientWidth()
	}
	return g.inner.Width()
}

// ---- status bar -----------------------------------------------------------------------------

type StatusBar struct {
	sb     *lcl.TStatusBar
	pb     *lcl.TPaintBox
	texts  []string
	widths []int32 // the first pane takes the rest
}

func newStatusBar(parent lcl.IWinControl, n int) *StatusBar {
	s := &StatusBar{texts: make([]string, n), widths: make([]int32, n)}
	if curSkin == nil {
		s.sb = lcl.NewStatusBar(parent)
		s.sb.SetParent(parent)
		s.sb.SetSimplePanel(false)
		for i := 0; i < n; i++ {
			s.sb.Panels().Add()
		}
		return s
	}
	s.pb = lcl.NewPaintBox(parent)
	s.pb.SetParent(parent)
	s.pb.SetAlign(types.AlBottom)
	s.pb.SetHeight(24)
	s.pb.SetOnPaint(func(lcl.IObject) { s.paint() })
	return s
}

func (s *StatusBar) paint() {
	c := s.pb.Canvas()
	w, h := s.pb.Width(), s.pb.Height()
	fill(c, 0, 0, w, h, curSkin.Face())
	used := int32(0)
	for i := 1; i < len(s.widths); i++ {
		used += s.widths[i] + 2
	}
	x := int32(2)
	first := w - used - 4 - 14
	for i := range s.texts {
		pw := s.widths[i]
		if i == 0 {
			pw = first
		}
		curSkin.StatusPane(c, rect(x, 2, x+pw, h-1), s.texts[i])
		x += pw + 2
	}
	// size grip: three diagonal line pairs
	for i := int32(0); i < 3; i++ {
		o := i * 4
		for k := int32(0); k <= 2-i; k++ { // diagonal strokes, Shadow then Highlight
			x, y := w-4-o-k*2, h-4-k*2
			fill(c, x, y+k, x+1, y+k+1, rgb(0x80, 0x80, 0x80))
		}
	}
}

func (s *StatusBar) SetText(i int, t string) {
	s.texts[i] = t
	if s.sb != nil {
		s.sb.Panels().Items(int32(i)).SetText(t)
	} else {
		s.pb.Invalidate()
	}
}

func (s *StatusBar) SetWidth(i int, w int32) {
	s.widths[i] = w
	if s.sb != nil {
		s.sb.Panels().Items(int32(i)).SetWidth(w)
	} else {
		s.pb.Invalidate()
	}
}

func (s *StatusBar) Width(i int) int32 { return s.widths[i] }

// ---- tab strip (skinned) over a PageControl whose own tabs are hidden --------------------------

type TabStrip struct {
	pb     *lcl.TPaintBox
	titles []string
	active int
	onSel  func(int)
}

func newTabStrip(parent lcl.IWinControl, n int, onSel func(int)) *TabStrip {
	t := &TabStrip{titles: make([]string, n), onSel: onSel}
	t.pb = lcl.NewPaintBox(parent)
	t.pb.SetParent(parent)
	t.pb.SetAlign(types.AlTop)
	t.pb.SetHeight(30)
	t.pb.SetOnPaint(func(lcl.IObject) {
		c := t.pb.Canvas()
		w, h := t.pb.Width(), t.pb.Height()
		fill(c, 0, 0, w, h, curSkin.Face())
		x := int32(10)
		for i, s := range t.titles {
			tw := c.TextWidth(strings.ReplaceAll(s, "&", "")) + 26
			if i != t.active {
				curSkin.Tab(c, rect(x, 4, x+tw, h), s, false)
			}
			x += tw + 2
		}
		x = 10
		for i, s := range t.titles {
			tw := c.TextWidth(strings.ReplaceAll(s, "&", "")) + 26
			if i == t.active {
				curSkin.Tab(c, rect(x, 4, x+tw, h), s, true)
			}
			x += tw + 2
		}
	})
	t.pb.SetOnMouseDown(func(_ lcl.IObject, _ types.TMouseButton, _ types.TShiftState, mx, my int32) {
		c := t.pb.Canvas()
		x := int32(10)
		for i, s := range t.titles {
			tw := c.TextWidth(strings.ReplaceAll(s, "&", "")) + 26
			if mx >= x && mx < x+tw {
				t.Select(i)
				return
			}
			x += tw + 2
		}
	})
	return t
}

func (t *TabStrip) Select(i int) {
	t.active = i
	t.pb.Invalidate()
	if t.onSel != nil {
		t.onSel(i)
	}
}

func (t *TabStrip) SetTitle(i int, s string) { t.titles[i] = s; t.pb.Invalidate() }

// ---- menu bar (skinned): titles painted by the skin, drop-downs are popup menus -----------------

type MenuBar struct {
	pb     *lcl.TPaintBox
	titles []string
	popups []*lcl.TPopupMenu
	hot    int
	owner  lcl.IWinControl
}

func newMenuBar(parent lcl.IWinControl, popups []*lcl.TPopupMenu) *MenuBar {
	m := &MenuBar{popups: popups, titles: make([]string, len(popups)), hot: -1}
	m.pb = lcl.NewPaintBox(parent)
	m.pb.SetParent(parent)
	m.pb.SetAlign(types.AlTop)
	m.pb.SetHeight(24)
	m.pb.SetOnPaint(func(lcl.IObject) {
		c := m.pb.Canvas()
		curSkin.MenuBar(c, rect(0, 0, m.pb.Width(), m.pb.Height()))
		x := int32(4)
		for i, s := range m.titles {
			tw := c.TextWidth(strings.ReplaceAll(s, "&", "")) + 20
			curSkin.MenuItem(c, rect(x, 1, x+tw, m.pb.Height()-1), s, i == m.hot)
			x += tw
		}
	})
	m.pb.SetOnMouseDown(func(_ lcl.IObject, _ types.TMouseButton, _ types.TShiftState, mx, my int32) {
		c := m.pb.Canvas()
		x := int32(4)
		for i, s := range m.titles {
			tw := c.TextWidth(strings.ReplaceAll(s, "&", "")) + 20
			if mx >= x && mx < x+tw {
				m.hot = i
				m.pb.Invalidate()
				p := m.pb.ClientToScreen(types.TPoint{X: x, Y: m.pb.Height()})
				m.popups[i].Popup(p.X, p.Y)
				m.hot = -1
				m.pb.Invalidate()
				return
			}
			x += tw
		}
	})
	return m
}

func (m *MenuBar) SetTitle(i int, s string) { m.titles[i] = s; m.pb.Invalidate() }

// ---- caption bar of the borderless main form -----------------------------------------------------

type CaptionBar struct {
	pb        *lcl.TPaintBox
	form      *lcl.TForm
	title     string
	drag      bool
	ox, oy    int32
	hotBtn    int
	maximised bool
}

func newCaptionBar(form *lcl.TForm) *CaptionBar {
	cb := &CaptionBar{form: form, hotBtn: -1}
	cb.pb = lcl.NewPaintBox(form)
	cb.pb.SetParent(form)
	cb.pb.SetAlign(types.AlTop)
	cb.pb.SetHeight(24)
	cb.pb.SetOnPaint(func(lcl.IObject) {
		r := rect(0, 0, cb.pb.Width(), cb.pb.Height())
		curSkin.Caption(cb.pb.Canvas(), r, cb.title, true, cb.hotBtn)
	})
	which := func(x, y int32) int {
		for i, b := range curSkin.CaptionButtons(rect(0, 0, cb.pb.Width(), cb.pb.Height())) {
			if x >= b.Left && x < b.Right && y >= b.Top && y < b.Bottom {
				return i
			}
		}
		return -1
	}
	cb.pb.SetOnMouseDown(func(_ lcl.IObject, _ types.TMouseButton, _ types.TShiftState, x, y int32) {
		if i := which(x, y); i >= 0 {
			cb.hotBtn = i
			cb.pb.Invalidate()
			return
		}
		cb.drag = true
		p := lcl.Mouse.CursorPos()
		cb.ox, cb.oy = p.X-form.Left(), p.Y-form.Top()
	})
	cb.pb.SetOnMouseMove(func(_ lcl.IObject, _ types.TShiftState, x, y int32) {
		if cb.drag {
			p := lcl.Mouse.CursorPos()
			form.SetLeft(p.X - cb.ox)
			form.SetTop(p.Y - cb.oy)
		}
	})
	cb.pb.SetOnMouseUp(func(_ lcl.IObject, _ types.TMouseButton, _ types.TShiftState, x, y int32) {
		cb.drag = false
		i := cb.hotBtn
		cb.hotBtn = -1
		cb.pb.Invalidate()
		if i >= 0 && which(x, y) == i {
			switch i {
			case 0:
				form.SetWindowState(types.WsMinimized)
			case 1:
				if cb.maximised {
					form.SetWindowState(types.WsNormal)
				} else {
					form.SetWindowState(types.WsMaximized)
				}
				cb.maximised = !cb.maximised
			case 2:
				form.Close()
			}
		}
	})
	return cb
}

func (cb *CaptionBar) SetTitle(s string) { cb.title = s; cb.pb.Invalidate() }
