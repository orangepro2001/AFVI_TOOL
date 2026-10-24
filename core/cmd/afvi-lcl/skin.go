package main

import (
	"strings"

	"github.com/energye/golcl/lcl"
	"github.com/energye/golcl/lcl/types"
	"github.com/energye/golcl/lcl/types/colors"
)

// Skin draws the look of the custom-drawn controls on a TCanvas. Two implementations:
// win2000 (the tokens of the win2000-style skill) and platinum (Mac OS 8/9). With -skin=native
// nothing is drawn by us and every control is the stock LCL control of the widgetset.
type Skin interface {
	Name() string
	Face() types.TColor
	Field() types.TColor
	Text() types.TColor
	TextDisabled() types.TColor
	SelectBG() types.TColor // list row selection
	SelectFG() types.TColor
	AltRow() types.TColor
	Panel(c *lcl.TCanvas, r types.TRect, sunken bool)
	Button(c *lcl.TCanvas, r types.TRect, caption string, st btnState)
	Group(c *lcl.TCanvas, r types.TRect, caption string)
	Tab(c *lcl.TCanvas, r types.TRect, caption string, active bool)
	TabPageEdge(c *lcl.TCanvas, r types.TRect)
	StatusPane(c *lcl.TCanvas, r types.TRect, text string)
	MenuBar(c *lcl.TCanvas, r types.TRect)
	MenuItem(c *lcl.TCanvas, r types.TRect, text string, hot bool)
	Caption(c *lcl.TCanvas, r types.TRect, title string, active bool, hotBtn int)
	// CaptionButtons returns the rectangles of min, max, close inside the caption bar.
	CaptionButtons(r types.TRect) [3]types.TRect
}

type btnState struct{ down, disabled, def, focus bool }

func rgb(r, g, b byte) types.TColor { return types.TColor(colors.RGB(r, g, b)) }

func rect(x1, y1, x2, y2 int32) types.TRect {
	return types.TRect{Left: x1, Top: y1, Right: x2, Bottom: y2}
}

// fill paints a rectangle [x1,x2) x [y1,y2) in one colour.
func fill(c *lcl.TCanvas, x1, y1, x2, y2 int32, col types.TColor) {
	if x2 <= x1 || y2 <= y1 {
		return
	}
	c.Brush().SetColor(col)
	c.Pen().SetColor(col)
	c.Rectangle(x1, y1, x2, y2)
}

func hline(c *lcl.TCanvas, x1, x2, y int32, col types.TColor) { fill(c, x1, y, x2, y+1, col) }
func vline(c *lcl.TCanvas, x, y1, y2 int32, col types.TColor) { fill(c, x, y1, x+1, y2, col) }

// ring draws a 1 px rectangle ring; tl is used for top and left, br for bottom and right (corner
// pixels belong to the top and bottom strips, as in the skin's recipes).
func ring(c *lcl.TCanvas, r types.TRect, tl, br types.TColor) {
	hline(c, r.Left, r.Right, r.Top, tl)
	vline(c, r.Left, r.Top+1, r.Bottom-1, tl)
	hline(c, r.Left, r.Right, r.Bottom-1, br)
	vline(c, r.Right-1, r.Top+1, r.Bottom-1, br)
}

func inset(r types.TRect, n int32) types.TRect {
	return rect(r.Left+n, r.Top+n, r.Right-n, r.Bottom-n)
}

// text draws caption text (vertically centred in r; horizontally by align -1/0/1).
func text(c *lcl.TCanvas, r types.TRect, s string, col types.TColor, align int, dx, dy int32) {
	s = strings.ReplaceAll(s, "&", "")
	c.Brush().SetStyle(types.BsClear)
	c.Font().SetColor(col)
	w, h := c.TextWidth(s), c.TextHeight(s)
	x := r.Left + 6
	switch align {
	case 0:
		x = r.Left + (r.Right-r.Left-w)/2
	case 1:
		x = r.Right - w - 6
	}
	c.TextOut(x+dx, r.Top+(r.Bottom-r.Top-h)/2+dy, s)
	c.Brush().SetStyle(types.BsSolid)
}

// ---- Windows 2000 -------------------------------------------------------------------

type win2000 struct{}

var (
	w2kFace      = rgb(0xD4, 0xD0, 0xC8)
	w2kHighlight = rgb(0xFF, 0xFF, 0xFF)
	w2kLight     = rgb(0xE8, 0xE5, 0xDE)
	w2kShadow    = rgb(0x80, 0x80, 0x80)
	w2kDkShadow  = rgb(0x40, 0x40, 0x40)
	w2kSelect    = rgb(0x0A, 0x24, 0x6A)
	w2kCapR      = rgb(0xA6, 0xCA, 0xF0)
)

func (win2000) Name() string               { return "win2000" }
func (win2000) Face() types.TColor         { return w2kFace }
func (win2000) Field() types.TColor        { return w2kHighlight }
func (win2000) Text() types.TColor         { return rgb(0, 0, 0) }
func (win2000) TextDisabled() types.TColor { return w2kShadow }
func (win2000) SelectBG() types.TColor     { return rgb(0xA6, 0xB8, 0xD6) } // soft blue: rows keep black text
func (win2000) SelectFG() types.TColor     { return rgb(0, 0, 0) }
func (win2000) AltRow() types.TColor       { return rgb(0xF4, 0xF3, 0xF1) }

func (win2000) raised(c *lcl.TCanvas, r types.TRect) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, w2kFace)
	ring(c, r, w2kHighlight, w2kDkShadow)
	ring(c, inset(r, 1), w2kLight, w2kShadow)
}

func (win2000) sunken(c *lcl.TCanvas, r types.TRect) {
	ring(c, r, w2kShadow, w2kHighlight)
	ring(c, inset(r, 1), w2kDkShadow, w2kLight)
}

func (s win2000) Panel(c *lcl.TCanvas, r types.TRect, sunken bool) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, w2kFace)
	if sunken {
		s.sunken(c, r)
	}
}

func (s win2000) Button(c *lcl.TCanvas, r types.TRect, caption string, st btnState) {
	d := int32(0)
	if st.down && !st.disabled {
		fill(c, r.Left, r.Top, r.Right, r.Bottom, w2kFace)
		ring(c, r, w2kDkShadow, w2kDkShadow)
		ring(c, inset(r, 1), w2kShadow, w2kShadow)
		d = 1
	} else {
		s.raised(c, r)
	}
	if st.disabled { // embossed: Highlight at +1,+1 under Shadow
		text(c, r, caption, w2kHighlight, 0, 1, 1)
		text(c, r, caption, w2kShadow, 0, 0, 0)
	} else {
		text(c, r, caption, s.Text(), 0, d, d)
	}
	if st.focus && !st.disabled { // dotted focus rectangle inset 4 px, drawn as alternating pixels
		f := inset(r, 4)
		for x := f.Left; x < f.Right; x += 2 {
			fill(c, x, f.Top, x+1, f.Top+1, s.Text())
			fill(c, x, f.Bottom-1, x+1, f.Bottom, s.Text())
		}
		for y := f.Top; y < f.Bottom; y += 2 {
			fill(c, f.Left, y, f.Left+1, y+1, s.Text())
			fill(c, f.Right-1, y, f.Right, y+1, s.Text())
		}
	}
}

func (s win2000) Group(c *lcl.TCanvas, r types.TRect, caption string) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, w2kFace)
	top := r.Top + 7
	e := rect(r.Left, top, r.Right-1, r.Bottom-1)
	ring(c, rect(e.Left+1, e.Top+1, e.Right+1, e.Bottom+1), w2kHighlight, w2kHighlight) // etched: Highlight ring +1,+1
	ring(c, e, w2kShadow, w2kShadow)                                                    // Shadow ring on top
	if caption != "" {
		c.Font().SetColor(s.Text())
		w := c.TextWidth(strings.ReplaceAll(caption, "&", ""))
		fill(c, r.Left+8, r.Top, r.Left+8+w+6, r.Top+16, w2kFace)
		text(c, rect(r.Left+8, r.Top, r.Left+8+w+6, r.Top+14), caption, s.Text(), 0, 0, 0)
	}
}

func (s win2000) Tab(c *lcl.TCanvas, r types.TRect, caption string, active bool) {
	rr := r
	if active {
		rr = rect(r.Left-2, r.Top-2, r.Right+2, r.Bottom+1)
	} else {
		rr.Top += 2
	}
	fill(c, rr.Left, rr.Top, rr.Right, rr.Bottom, w2kFace)
	vline(c, rr.Left, rr.Top, rr.Bottom, w2kHighlight)
	hline(c, rr.Left, rr.Right, rr.Top, w2kHighlight)
	vline(c, rr.Left+1, rr.Top+1, rr.Bottom, w2kLight)
	hline(c, rr.Left+1, rr.Right-1, rr.Top+1, w2kLight)
	vline(c, rr.Right-1, rr.Top, rr.Bottom, w2kDkShadow)
	vline(c, rr.Right-2, rr.Top+1, rr.Bottom, w2kShadow)
	text(c, rr, caption, s.Text(), 0, 0, 0)
}

func (s win2000) TabPageEdge(c *lcl.TCanvas, r types.TRect) {
	ring(c, r, w2kHighlight, w2kDkShadow)
	ring(c, inset(r, 1), w2kLight, w2kShadow)
}

func (s win2000) StatusPane(c *lcl.TCanvas, r types.TRect, t string) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, w2kFace)
	ring(c, r, w2kShadow, w2kHighlight) // the single 1 px status ring
	text(c, rect(r.Left+1, r.Top, r.Right-4, r.Bottom), t, s.Text(), -1, 0, 0)
}

func (s win2000) MenuBar(c *lcl.TCanvas, r types.TRect) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, w2kFace)
}

func (s win2000) MenuItem(c *lcl.TCanvas, r types.TRect, t string, hot bool) {
	if hot {
		fill(c, r.Left, r.Top, r.Right, r.Bottom, w2kSelect)
		text(c, r, t, rgb(255, 255, 255), 0, 0, 0)
	} else {
		text(c, r, t, s.Text(), 0, 0, 0)
	}
}

func (s win2000) CaptionButtons(r types.TRect) [3]types.TRect {
	var b [3]types.TRect
	sz := r.Bottom - r.Top - 6
	x := r.Right - 3 - sz
	b[2] = rect(x, r.Top+3, x+sz, r.Top+3+sz)
	x -= sz + 2
	b[1] = rect(x-2, r.Top+3, x-2+sz, r.Top+3+sz)
	x -= sz
	b[0] = rect(x-2, r.Top+3, x-2+sz, r.Top+3+sz)
	return b
}

func (s win2000) Caption(c *lcl.TCanvas, r types.TRect, title string, active bool, hotBtn int) {
	// gradient from CaptionL to CaptionR (the only gradient of the skin); inactive: Shadow to Face
	l, rr := w2kSelect, w2kCapR
	if !active {
		l, rr = w2kShadow, w2kFace
	}
	w := r.Right - r.Left
	lr, lg, lb := int32(l&0xFF), int32((l>>8)&0xFF), int32((l>>16)&0xFF)
	rr0, rg, rb := int32(rr&0xFF), int32((rr>>8)&0xFF), int32((rr>>16)&0xFF)
	for x := int32(0); x < w; x += 2 {
		mix := func(a, b int32) byte { return byte(a + (b-a)*x/w) }
		fill(c, r.Left+x, r.Top, r.Left+x+2, r.Bottom, rgb(mix(lr, rr0), mix(lg, rg), mix(lb, rb)))
	}
	// bold look: the title drawn twice, 1 px apart
	text(c, rect(r.Left+2, r.Top, r.Right, r.Bottom), title, rgb(255, 255, 255), -1, 0, 0)
	text(c, rect(r.Left+2, r.Top, r.Right, r.Bottom), title, rgb(255, 255, 255), -1, 1, 0)
	for i, b := range s.CaptionButtons(r) {
		st := btnState{down: i == hotBtn}
		s.Button(c, b, [3]string{"_", "[]", "x"}[i], st)
	}
}

// ---- Platinum (Mac OS 8/9) --------------------------------------------------------------
//
// Drawn from memory of the Mac OS 8/9 appearance, not from a reference screenshot: face #DDDDDD,
// white / #888888 bevels, black 1 px outlines, rounded push buttons with a light-to-dark gradient,
// a double outline on the default button, pinstriped title bars with a close box on the left and a
// zoom box on the right. The highlight colour is a guess (lavender).

type platinum struct{}

var (
	pFace   = rgb(0xDD, 0xDD, 0xDD)
	pWhite  = rgb(0xFF, 0xFF, 0xFF)
	pGray   = rgb(0x88, 0x88, 0x88)
	pDark   = rgb(0x55, 0x55, 0x55)
	pBlack  = rgb(0, 0, 0)
	pStripe = rgb(0x99, 0x99, 0x99)
	pHilite = rgb(0xA3, 0xA3, 0xD6)
)

func (platinum) Name() string               { return "platinum" }
func (platinum) Face() types.TColor         { return pFace }
func (platinum) Field() types.TColor        { return pWhite }
func (platinum) Text() types.TColor         { return pBlack }
func (platinum) TextDisabled() types.TColor { return rgb(0x88, 0x88, 0x88) }
func (platinum) SelectBG() types.TColor     { return pHilite }
func (platinum) SelectFG() types.TColor     { return pBlack }
func (platinum) AltRow() types.TColor       { return rgb(0xEE, 0xEE, 0xEE) }

func (platinum) Panel(c *lcl.TCanvas, r types.TRect, sunken bool) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, pFace)
	if sunken {
		ring(c, r, pGray, pWhite)
		ring(c, inset(r, 1), pDark, pFace)
	}
}

func (platinum) Button(c *lcl.TCanvas, r types.TRect, caption string, st btnState) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, pFace)
	b := r
	if st.def && !st.disabled { // default button: an extra black ring outside the button
		c.Pen().SetColor(pBlack)
		c.Brush().SetStyle(types.BsClear)
		c.RoundRect(r.Left, r.Top, r.Right, r.Bottom, 10, 10)
		c.Brush().SetStyle(types.BsSolid)
		b = inset(r, 2)
	}
	// body: vertical gradient white -> #BBBBBB (pressed: #999999 -> #CCCCCC)
	h := b.Bottom - b.Top
	for y := int32(0); y < h; y++ {
		var v byte
		if st.down && !st.disabled {
			v = byte(0x99 + int32(0x33)*y/h)
		} else {
			v = byte(0xFF - int32(0x44)*y/h)
		}
		hline(c, b.Left+2, b.Right-2, b.Top+y, rgb(v, v, v))
	}
	if !st.down {
		hline(c, b.Left+3, b.Right-3, b.Top+1, pWhite)
		vline(c, b.Left+1, b.Top+3, b.Bottom-3, pWhite)
		hline(c, b.Left+3, b.Right-3, b.Bottom-2, pGray)
		vline(c, b.Right-2, b.Top+3, b.Bottom-3, pGray)
	}
	c.Pen().SetColor(pBlack)
	c.Brush().SetStyle(types.BsClear)
	c.RoundRect(b.Left, b.Top, b.Right, b.Bottom, 8, 8)
	c.Brush().SetStyle(types.BsSolid)
	if st.disabled {
		text(c, b, caption, pGray, 0, 0, 0)
	} else {
		text(c, b, caption, pBlack, 0, 0, 0)
	}
}

func (platinum) Group(c *lcl.TCanvas, r types.TRect, caption string) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, pFace)
	top := r.Top + 7
	// a thin gray line with a white line below it, the Platinum "well"
	ring(c, rect(r.Left+1, top+1, r.Right, r.Bottom), pWhite, pWhite)
	ring(c, rect(r.Left, top, r.Right-1, r.Bottom-1), pGray, pGray)
	if caption != "" {
		w := c.TextWidth(strings.ReplaceAll(caption, "&", ""))
		fill(c, r.Left+8, r.Top, r.Left+8+w+8, r.Top+16, pFace)
		text(c, rect(r.Left+8, r.Top, r.Left+8+w+8, r.Top+14), caption, pBlack, 0, 0, 0)
	}
}

func (platinum) Tab(c *lcl.TCanvas, r types.TRect, caption string, active bool) {
	rr := r
	if !active {
		rr.Top += 3
	}
	fill(c, rr.Left, rr.Top, rr.Right, rr.Bottom+1, pFace)
	c.Pen().SetColor(pBlack)
	c.Brush().SetStyle(types.BsClear)
	c.RoundRect(rr.Left, rr.Top, rr.Right, rr.Bottom+8, 8, 8) // rounded top, the bottom is overdrawn below
	c.Brush().SetStyle(types.BsSolid)
	if active {
		fill(c, rr.Left+1, rr.Bottom-1, rr.Right-1, rr.Bottom+2, pFace)
	} else {
		fill(c, rr.Left+2, rr.Top+1, rr.Right-2, rr.Bottom-1, rgb(0xC8, 0xC8, 0xC8))
	}
	text(c, rr, caption, pBlack, 0, 0, 0)
}

func (platinum) TabPageEdge(c *lcl.TCanvas, r types.TRect) {
	ring(c, r, pBlack, pBlack)
	ring(c, inset(r, 1), pWhite, pGray)
}

func (platinum) StatusPane(c *lcl.TCanvas, r types.TRect, t string) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, pFace)
	ring(c, r, pGray, pWhite)
	text(c, rect(r.Left+1, r.Top, r.Right-4, r.Bottom), t, pBlack, -1, 0, 0)
}

func (platinum) MenuBar(c *lcl.TCanvas, r types.TRect) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, pFace)
	hline(c, r.Left, r.Right, r.Bottom-1, pBlack)
}

func (platinum) MenuItem(c *lcl.TCanvas, r types.TRect, t string, hot bool) {
	if hot {
		fill(c, r.Left, r.Top, r.Right, r.Bottom-1, pBlack)
		text(c, r, t, pWhite, 0, 0, 0)
	} else {
		text(c, r, t, pBlack, 0, 0, 0)
	}
}

func (platinum) CaptionButtons(r types.TRect) [3]types.TRect {
	var b [3]types.TRect
	sz := int32(13)
	y := r.Top + (r.Bottom-r.Top-sz)/2
	b[2] = rect(r.Left+8, y, r.Left+8+sz, y+sz)           // close box on the left
	b[1] = rect(r.Right-8-sz, y, r.Right-8, y+sz)         // zoom box on the right
	b[0] = rect(r.Right-8-2*sz-4, y, r.Right-12-sz, y+sz) // collapse box
	return b
}

func (s platinum) Caption(c *lcl.TCanvas, r types.TRect, title string, active bool, hotBtn int) {
	fill(c, r.Left, r.Top, r.Right, r.Bottom, pFace)
	if active { // pinstripes: 1 px lines every 2 px
		for y := r.Top + 3; y < r.Bottom-3; y += 2 {
			hline(c, r.Left+2, r.Right-2, y, pStripe)
			hline(c, r.Left+2, r.Right-2, y+1, pWhite)
		}
	}
	c.Pen().SetColor(pBlack)
	c.Brush().SetStyle(types.BsClear)
	c.Rectangle(r.Left, r.Top, r.Right, r.Bottom)
	c.Brush().SetStyle(types.BsSolid)
	tw := c.TextWidth(title)
	cx := r.Left + (r.Right-r.Left-tw)/2
	fill(c, cx-8, r.Top+2, cx+tw+8, r.Bottom-2, pFace)
	text(c, rect(cx-8, r.Top, cx+tw+8, r.Bottom), title, pBlack, 0, 0, 0)
	for i, b := range s.CaptionButtons(r) {
		fill(c, b.Left, b.Top, b.Right, b.Bottom, pFace)
		ring(c, b, pBlack, pBlack)
		if i == hotBtn {
			fill(c, b.Left+2, b.Top+2, b.Right-2, b.Bottom-2, pGray)
		} else {
			ring(c, inset(b, 1), pWhite, pGray)
		}
	}
}

// curSkin is nil for -skin=native.
var curSkin Skin

func pickSkin(name string) {
	switch name {
	case "win2000":
		curSkin = win2000{}
	case "platinum":
		curSkin = platinum{}
	default:
		curSkin = nil
	}
}
