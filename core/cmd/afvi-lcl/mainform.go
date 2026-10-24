package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/ops"
	"afvi/tools/core/internal/policy"
	"afvi/tools/core/internal/service"

	"github.com/energye/golcl/lcl"
	"github.com/energye/golcl/lcl/types"
)

var nodeKinds = []string{"FM1", "FM2", "BM"}

// TMainForm is the main window: Model Copy and Export Data To Excel tabs, menu and status bar.
// Everything is a stock LCL control, so on Windows it is the native Win32 look and nothing else.
type TMainForm struct {
	*lcl.TForm
	app *App

	cap      *CaptionBar
	mbar     *MenuBar
	tabs     *TabStrip
	popups   []*lcl.TPopupMenu
	mFonts   map[int32]*lcl.TMenuItem
	mFont    *lcl.TMenuItem
	fontSize int32
	applying bool // set while texts are written: radio menu items fire OnClick when their Checked changes

	menu                                                       *lcl.TMainMenu
	mFile, mMachines, mRefresh, mExit                          *lcl.TMenuItem
	mView, mLang, mTools, mExport, mHelp, mAbout, mSep1, mSep2 *lcl.TMenuItem
	mLangs                                                     map[string]*lcl.TMenuItem

	pageCopy, pageExport   lcl.IWinControl // parents of the two pages: tab sheets (native) or plain panels (skinned)
	panelCopy, panelExport *lcl.TPanel
	pages                  *lcl.TPageControl
	tabCopy, tabExport     *lcl.TTabSheet
	status                 *StatusBar

	// source
	gbSource                       *Grp
	lblMachine, lblNode, lblSearch *lcl.TLabel
	cbMachine, cbNode              *lcl.TComboBox
	edSearch                       *lcl.TEdit
	btnRefresh                     *Btn
	lv                             *lcl.TListView

	// operations
	gbCopy                           *Grp
	lblSourceIs, lblTN, lblTM, lblNN *lcl.TLabel
	rbSingle, rbGroup                *lcl.TRadioButton
	cbSingle, cbGroup                *lcl.TComboBox
	edCopyName                       *lcl.TEdit
	lblNoteCopy                      *lcl.TLabel
	btnCopy                          *Btn
	gbRename                         *Grp
	chkRenameGroup                   *lcl.TCheckBox
	lblRN                            *lcl.TLabel
	edRenameName                     *lcl.TEdit
	btnRename                        *Btn
	gbRules                          *Grp
	lblRules                         [4]*lcl.TLabel

	// export tab
	gbExport  *Grp
	lblExport *lcl.TLabel
	btnExport *Btn

	// state
	machines   []machine.Machine
	rows       []service.ModelRow
	view       []service.ModelRow // filtered and sorted
	singles    []policy.Ref
	groups     []string
	selected   string
	populating bool
	sortCol    int
	sortDesc   bool
	loadSeq    int
	busy       bool
	readOnly   bool
	lockOpen   bool
	statusKey  string
	statusArgs []any
	repository string
}

func (f *TMainForm) T(key string, args ...any) string { return f.app.T(key, args...) }

// ---- construction -------------------------------------------------------------

func (f *TMainForm) OnFormCreate(sender lcl.IObject) {
	f.app = theApp
	f.SetWidth(1060)
	f.SetHeight(720)
	f.Constraints().SetMinWidth(900)
	f.Constraints().SetMinHeight(600)
	f.SetPosition(types.PoScreenCenter)
	f.app.initDefer(f)

	// Align=Top stacks the controls in reverse creation order: tabs first, caption last = caption on top.
	if curSkin != nil {
		f.SetBorderStyle(types.BsNone)
		f.SetColor(curSkin.Face())
		f.tabs = newTabStrip(f, 2, func(i int) { f.showPage(i) })
	}
	f.buildMenu()
	if curSkin != nil {
		f.cap = newCaptionBar(f.TForm)
	}
	f.status = newStatusBar(f, 4)
	f.status.SetWidth(1, 300)
	f.status.SetWidth(2, 110)
	f.status.SetWidth(3, 270)

	if curSkin == nil {
		f.pages = lcl.NewPageControl(f)
		f.pages.SetParent(f)
		f.pages.SetAlign(types.AlClient)
		f.tabCopy = lcl.NewTabSheet(f.pages)
		f.tabCopy.SetPageControl(f.pages)
		f.tabExport = lcl.NewTabSheet(f.pages)
		f.tabExport.SetPageControl(f.pages)
		f.pageCopy, f.pageExport = f.tabCopy, f.tabExport
	} else { // two plain panels instead of a PageControl: hidden tabs would hide their pages too
		holder := lcl.NewPanel(f)
		holder.SetParent(f)
		holder.SetBevelOuter(types.BvNone)
		holder.SetAlign(types.AlClient)
		holder.SetColor(curSkin.Face())
		mk := func() *lcl.TPanel {
			p := lcl.NewPanel(holder)
			p.SetParent(holder)
			p.SetBevelOuter(types.BvNone)
			p.SetAlign(types.AlClient)
			p.SetColor(curSkin.Face())
			return p
		}
		f.panelCopy, f.panelExport = mk(), mk()
		f.panelExport.SetVisible(false)
		f.pageCopy, f.pageExport = f.panelCopy, f.panelExport
	}

	f.buildCopyTab()
	f.buildExportTab()

	f.SetOnResize(func(lcl.IObject) { f.layout() })
	shown := false
	f.SetOnShow(func(lcl.IObject) {
		if !shown { // the widgetset ignores the size set before the window exists
			shown = true
			t := lcl.NewTimer(f) // a size set during OnShow is overwritten by the widgetset: set it a moment later
			t.SetInterval(100)
			t.SetOnTimer(func(lcl.IObject) {
				t.SetEnabled(false)
				x, y := (lcl.Screen.Width()-1060)/2, (lcl.Screen.Height()-720)/2
				if x < 0 {
					x = 0
				}
				if y < 0 {
					y = 0
				}
				f.SetBounds(x, y, 1060, 720)
				f.layout()
			})
		}
		f.layout()
	})
	f.applyTexts()
	f.setStatus("st_ready")
	if f.app.fontPt > 0 {
		f.setFontSize(f.app.fontPt)
	}
	f.initialize()
	if theSelf != nil {
		theSelf.run(f)
	}
	if name := os.Getenv("AFVI_LCL_SELECT"); name != "" { // for screenshots: pick a row once the list is loaded
		t := lcl.NewTimer(f)
		t.SetInterval(1500)
		t.SetOnTimer(func(lcl.IObject) { t.SetEnabled(false); selectRow(f, name) })
	}
}

func (f *TMainForm) buildMenu() {
	var tops [4]*lcl.TMenuItem
	if curSkin == nil {
		f.menu = lcl.NewMainMenu(f)
		for i := range tops {
			tops[i] = lcl.NewMenuItem(f)
			f.menu.Items().Add(tops[i])
		}
		f.SetMenu(f.menu)
	} else { // the drop-downs are popup menus under a painted bar
		for i := range tops {
			p := lcl.NewPopupMenu(f)
			f.popups = append(f.popups, p)
			tops[i] = p.Items()
		}
		f.mbar = newMenuBar(f, f.popups)
	}
	add := func(parent *lcl.TMenuItem, onClick func()) *lcl.TMenuItem {
		it := lcl.NewMenuItem(f)
		if onClick != nil {
			it.SetOnClick(func(lcl.IObject) { onClick() })
		}
		parent.Add(it)
		return it
	}
	f.mFile, f.mView, f.mTools, f.mHelp = tops[0], tops[1], tops[2], tops[3]
	f.mMachines = add(f.mFile, f.editMachines)
	f.mRefresh = add(f.mFile, func() { f.reload(false) })
	f.mSep1 = add(f.mFile, nil)
	f.mSep1.SetCaption("-")
	f.mExit = add(f.mFile, func() { f.Close() })

	f.mLang = add(f.mView, nil)
	f.mLangs = map[string]*lcl.TMenuItem{}
	for _, tag := range langTags {
		tag := tag
		it := add(f.mLang, func() { f.setLanguage(tag) })
		it.SetCaption(f.app.loc.Name(tag))
		it.SetRadioItem(true)
		it.SetGroupIndex(1)
		f.mLangs[tag] = it
	}
	f.mFont = add(f.mView, nil)
	f.mFonts = map[int32]*lcl.TMenuItem{}
	for _, n := range []int32{10, 11, 12, 14} { // LCL point sizes: 14 pt is about 18 px
		n := n
		it := add(f.mFont, func() { f.setFontSize(n) })
		it.SetCaption(strconv.Itoa(int(n)))
		it.SetRadioItem(true)
		it.SetGroupIndex(2)
		f.mFonts[n] = it
	}
	f.mExport = add(f.mTools, f.exportExcel)
	f.mAbout = add(f.mHelp, f.about)
}

func (f *TMainForm) showPage(i int) {
	f.panelCopy.SetVisible(i == 0)
	f.panelExport.SetVisible(i == 1)
}

// tabCaption is the caption of tab i, whichever control shows it.
func (f *TMainForm) tabCaption(i int) string {
	if f.tabs != nil {
		return f.tabs.titles[i]
	}
	return []string{f.tabCopy.Caption(), f.tabExport.Caption()}[i]
}

func (f *TMainForm) setFontSize(n int32) {
	if f.applying {
		return
	}
	f.fontSize = n
	f.Font().SetSize(n)
	for k, it := range f.mFonts {
		it.SetChecked(k == n)
	}
	f.layout()
	for _, c := range []lcl.IWinControl{f.tabCopy, f.tabExport} {
		_ = c
	}
}

// gb is the usual group box in the right column.
func (f *TMainForm) buildCopyTab() {
	left := lcl.NewPanel(f.pageCopy)
	if curSkin != nil {
		left.SetColor(curSkin.Face())
	}
	left.SetParent(f.pageCopy)
	left.SetBevelOuter(types.BvNone)
	left.SetAlign(types.AlClient)
	left.SetBorderWidth(6)

	right := lcl.NewScrollBox(f.pageCopy)
	if curSkin != nil {
		right.SetColor(curSkin.Face())
	}
	right.SetParent(f.pageCopy)
	right.SetAlign(types.AlRight)
	right.SetWidth(366)
	right.SetBorderStyle(types.BsNone)
	right.SetAutoScroll(true)

	// source group + list
	f.gbSource = newGroup(left, 0, 0, 200, 56)
	f.gbSource.SetAlign(types.AlTop)
	f.gbSource.SetHeight(56)
	f.lblMachine = newLabel(f.gbSource.P(), "", 10, 12)
	f.cbMachine = newCombo(f.gbSource.P(), 0, 6, 150)
	f.lblNode = newLabel(f.gbSource.P(), "", 0, 12)
	f.cbNode = newCombo(f.gbSource.P(), 0, 6, 80)
	for _, n := range nodeKinds {
		f.cbNode.Items().Add(n)
	}
	f.cbNode.SetItemIndex(0)
	f.lblSearch = newLabel(f.gbSource.P(), "", 0, 12)
	f.edSearch = newEdit(f.gbSource.P(), 0, 6, 150)
	f.btnRefresh = newButton(f.gbSource.P(), 0, 4, 80, 26, func() { f.reload(false) })

	spacer := lcl.NewPanel(left)
	spacer.SetParent(left)
	spacer.SetBevelOuter(types.BvNone)
	spacer.SetAlign(types.AlTop)
	spacer.SetHeight(6)
	spacer.SetTop(100)

	f.lv = lcl.NewListView(left)
	f.lv.SetParent(left)
	f.lv.SetAlign(types.AlClient)
	f.lv.SetViewStyle(types.VsReport)
	f.lv.SetReadOnly(true)
	f.lv.SetRowSelect(true)
	f.lv.SetGridLines(false)
	f.lv.SetColumnClick(true)
	f.lv.SetMultiSelect(false)
	for _, w := range []int32{190, 70, 112, 50, 124, 104, 150} {
		f.lv.Columns().Add().SetWidth(w)
	}
	if curSkin != nil {
		f.lv.SetColor(curSkin.Field())
		f.lv.SetOwnerDraw(true)
		f.lv.SetOnDrawItem(f.drawRow)
	}
	f.lv.SetOnColumnClick(func(_ lcl.IObject, c *lcl.TListColumn) { f.sortBy(int(c.Index())) })
	f.lv.SetOnSelectItem(func(_ lcl.IObject, it *lcl.TListItem, sel bool) { f.onSelect() })

	f.cbMachine.SetOnChange(func(lcl.IObject) { f.onSourceChanged() })
	f.cbNode.SetOnChange(func(lcl.IObject) { f.onSourceChanged() })
	f.edSearch.SetOnChange(func(lcl.IObject) { f.applyFilter() })

	// copy
	f.gbCopy = newGroup(right, 4, 0, 340, 348)
	f.lblSourceIs = newLabel(f.gbCopy.P(), "", 10, 6)
	f.rbSingle = lcl.NewRadioButton(f.gbCopy.P())
	f.rbSingle.SetParent(f.gbCopy.P())
	f.rbSingle.SetBounds(10, 28, 316, 22)
	f.rbSingle.SetChecked(true)
	f.lblTN = newLabel(f.gbCopy.P(), "", 28, 54)
	f.cbSingle = newCombo(f.gbCopy.P(), 28, 72, 290)
	f.rbGroup = lcl.NewRadioButton(f.gbCopy.P())
	f.rbGroup.SetParent(f.gbCopy.P())
	f.rbGroup.SetBounds(10, 102, 316, 22)
	f.lblTM = newLabel(f.gbCopy.P(), "", 28, 128)
	f.cbGroup = newCombo(f.gbCopy.P(), 28, 146, 290)
	f.lblNN = newLabel(f.gbCopy.P(), "", 10, 180)
	f.edCopyName = newEdit(f.gbCopy.P(), 10, 198, 308)
	f.lblNoteCopy = wrapLabel(f.gbCopy.P(), 10, 232, 308, 56)
	f.btnCopy = newButton(f.gbCopy.P(), 214, 294, 104, 28, f.doCopy)
	f.rbSingle.SetOnClick(func(lcl.IObject) { f.updateEnabled() })
	f.rbGroup.SetOnClick(func(lcl.IObject) { f.updateEnabled() })
	f.cbSingle.SetOnChange(func(lcl.IObject) { f.updateEnabled() })
	f.cbGroup.SetOnChange(func(lcl.IObject) { f.updateEnabled() })

	// rename
	f.gbRename = newGroup(right, 4, 356, 340, 158)
	f.chkRenameGroup = lcl.NewCheckBox(f.gbRename.P())
	f.chkRenameGroup.SetParent(f.gbRename.P())
	f.chkRenameGroup.SetBounds(10, 6, 316, 22)
	f.lblRN = newLabel(f.gbRename.P(), "", 10, 38)
	f.edRenameName = newEdit(f.gbRename.P(), 10, 56, 308)
	f.btnRename = newButton(f.gbRename.P(), 214, 92, 104, 28, f.doRename)

	// rules
	f.gbRules = newGroup(right, 4, 522, 340, 136)
	for i := range f.lblRules {
		f.lblRules[i] = wrapLabel(f.gbRules.P(), 10, 4+int32(i)*30, 318, 30)
	}
}

// drawRow paints a list row with the skin's colours (the header stays the widgetset's).
func (f *TMainForm) drawRow(lv *lcl.TListView, it *lcl.TListItem, r types.TRect, st types.TOwnerDrawState) {
	c := lv.Canvas()
	bg, fg := curSkin.Field(), curSkin.Text()
	if it.Index()%2 == 1 {
		bg = curSkin.AltRow()
	}
	if it.Selected() {
		bg, fg = curSkin.SelectBG(), curSkin.SelectFG()
	}
	fill(c, r.Left, r.Top, lv.ClientWidth(), r.Bottom, bg)
	c.Brush().SetStyle(types.BsClear)
	c.Font().SetColor(fg)
	x := r.Left
	cells := append([]string{it.Caption()}, it.SubItems().Text())
	_ = cells
	n := lv.Columns().Count()
	for i := int32(0); i < n; i++ {
		txt := ""
		if i == 0 {
			txt = it.Caption()
		} else if i-1 < it.SubItems().Count() {
			txt = it.SubItems().Strings(i - 1)
		}
		w := lv.Column(i).Width()
		c.TextOut(x+6, r.Top+(r.Bottom-r.Top-c.TextHeight(txt))/2, txt)
		x += w
	}
	c.Brush().SetStyle(types.BsSolid)
}

func (f *TMainForm) buildExportTab() {
	var host lcl.IWinControl = f.pageExport
	f.gbExport = newGroup(host, 12, 12, 600, 170)
	f.lblExport = wrapLabel(f.gbExport.P(), 12, 8, 570, 76)
	f.btnExport = newButton(f.gbExport.P(), 12, 96, 120, 28, f.exportExcel)
}

// ---- text and layout ----------------------------------------------------------

func (f *TMainForm) applyTexts() {
	f.applying = true
	defer func() { f.applying = false }()
	if os.Getenv("AFVI_LCL_DEBUG") != "" {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "applyTexts panic: %v\n%s\n", r, debug.Stack())
				panic(r)
			}
		}()
	}
	f.SetCaption("AFVI TOOLS")
	lcl.Application.SetTitle("AFVI TOOLS")
	if f.cap != nil {
		f.cap.SetTitle("AFVI TOOLS " + f.app.svc.Version)
		f.mbar.SetTitle(0, f.T("menu_file"))
		f.mbar.SetTitle(1, f.T("menu_view"))
		f.mbar.SetTitle(2, f.T("menu_tools"))
		f.mbar.SetTitle(3, f.T("menu_help"))
		f.tabs.SetTitle(0, f.T("tab_copy"))
		f.tabs.SetTitle(1, f.T("tab_export"))
	}
	f.mFont.SetCaption(f.T("menu_fontsize"))
	f.mFile.SetCaption(f.T("menu_file"))
	f.mMachines.SetCaption(f.T("menu_machines"))
	f.mRefresh.SetCaption(f.T("menu_refresh"))
	f.mExit.SetCaption(f.T("menu_exit"))
	f.mView.SetCaption(f.T("menu_view"))
	f.mLang.SetCaption(f.T("menu_language"))
	f.mTools.SetCaption(f.T("menu_tools"))
	f.mExport.SetCaption(f.T("menu_export"))
	f.mHelp.SetCaption(f.T("menu_help"))
	f.mAbout.SetCaption(f.T("menu_about"))
	for tag, it := range f.mLangs {
		it.SetChecked(tag == f.app.loc.Tag())
	}
	for n, it := range f.mFonts {
		it.SetChecked(n == f.fontSize)
	}

	if f.tabCopy != nil {
		f.tabCopy.SetCaption(f.T("tab_copy"))
		f.tabExport.SetCaption(f.T("tab_export"))
	}
	f.gbSource.SetCaption(f.T("gb_source"))
	f.lblMachine.SetCaption(f.T("lbl_machine"))
	f.lblNode.SetCaption(f.T("lbl_node"))
	f.lblSearch.SetCaption(f.T("lbl_search"))
	f.edSearch.SetTextHint(f.T("search_hint"))
	f.btnRefresh.SetCaption(f.T("btn_refresh"))
	heads := []string{f.T("col_model"), f.T("col_class"), "PxRepository", ".his", "INSPECT_SPEC", "LIGHT_SPEC", f.T("col_modified")}
	for i, h := range heads {
		f.lv.Columns().Items(int32(i)).SetCaption(h)
	}

	f.gbCopy.SetCaption(f.T("gb_copy"))
	f.rbSingle.SetCaption(f.T("rb_single"))
	f.lblTN.SetCaption(f.T("lbl_target_node"))
	f.rbGroup.SetCaption(f.T("rb_group"))
	f.lblTM.SetCaption(f.T("lbl_target_machine"))
	f.lblNN.SetCaption(f.T("lbl_newname_opt"))
	f.edCopyName.SetTextHint(f.T("hint_keep_name"))
	f.lblNoteCopy.SetCaption(f.T("note_copy"))
	f.btnCopy.SetCaption(f.T("btn_copy"))
	f.gbRename.SetCaption(f.T("gb_rename"))
	f.chkRenameGroup.SetCaption(f.T("chk_rename_group"))
	f.lblRN.SetCaption(f.T("lbl_newname"))
	f.btnRename.SetCaption(f.T("btn_rename"))
	f.gbRules.SetCaption(f.T("gb_rules"))
	for i, l := range f.lblRules {
		l.SetCaption(f.T("rule_" + strconv.Itoa(i+1)))
	}
	f.gbExport.SetCaption(f.T("gb_export"))
	f.lblExport.SetCaption(f.T("export_page_text"))
	f.btnExport.SetCaption(f.T("btn_export"))

	f.updateSourceLabel()
	f.fillList()
	f.updateStatusPanels()
	f.layout()
}

// layout places the controls of the source row and sizes the status panels.
func (f *TMainForm) layout() {
	if f.gbSource == nil {
		return
	}
	x := int32(10)
	place := func(l *lcl.TLabel, c lcl.IControl, cw int32) {
		l.SetLeft(x)
		x += l.Width() + 6
		c.SetLeft(x)
		if cw > 0 {
			c.SetWidth(cw)
			x += cw + 16
		}
	}
	place(f.lblMachine, f.cbMachine, 150)
	place(f.lblNode, f.cbNode, 80)
	f.lblSearch.SetLeft(x)
	x += f.lblSearch.Width() + 6
	bw := f.btnRefresh.Width()
	gw := f.gbSource.ClientWidth()
	f.edSearch.SetLeft(x)
	ew := gw - x - bw - 18
	if ew < 80 {
		ew = 80
	}
	f.edSearch.SetWidth(ew)
	f.btnRefresh.SetLeft(x + ew + 8)

	if f.status != nil {
		used := f.status.Width(1) + f.status.Width(2) + f.status.Width(3)
		w := f.ClientWidth() - used - 24
		if w < 100 {
			w = 100
		}
		f.status.SetWidth(0, w)
	}
}

func (f *TMainForm) setStatus(key string, args ...any) {
	f.statusKey, f.statusArgs = key, args
	f.status.SetText(0, f.T(key, args...))
}

func (f *TMainForm) setStatusText(text string) {
	f.statusKey = ""
	f.status.SetText(0, text)
}

func (f *TMainForm) lockText() string {
	if f.readOnly {
		if f.lockOpen {
			return f.T("lock_open")
		}
		return f.T("lock_locked")
	}
	return f.T("lock_writable")
}

func (f *TMainForm) updateStatusPanels() {
	if f.statusKey != "" {
		f.status.SetText(0, f.T(f.statusKey, f.statusArgs...))
	}
	f.status.SetText(1, f.repository)
	f.status.SetText(3, f.lockText())
	q := strings.TrimSpace(f.edSearch.Text())
	if len(f.rows) == 0 && f.repository == "" {
		f.status.SetText(2, "")
	} else if q == "" {
		f.status.SetText(2, f.T("count_models", len(f.rows)))
	} else {
		f.status.SetText(2, f.T("count_filtered", len(f.view), len(f.rows)))
	}
}

func (f *TMainForm) setLanguage(tag string) {
	if f.applying {
		return
	}
	f.app.loc.Set(tag)
	saveLanguage(tag)
	f.applyTexts()
}

// ---- data ---------------------------------------------------------------------

func (f *TMainForm) curMachine() string {
	i := int(f.cbMachine.ItemIndex())
	if i < 0 || i >= len(f.machines) {
		return ""
	}
	return f.machines[i].Name
}

func (f *TMainForm) curNode() string {
	i := int(f.cbNode.ItemIndex())
	if i < 0 {
		i = 0
	}
	return nodeKinds[i]
}

func (f *TMainForm) initialize() {
	h := f.app.svc.Health(f.app.ctx())
	f.lockOpen = h.AllowNetworkWrite
	f.readOnly = true
	f.loadMachines("")
}

func (f *TMainForm) loadMachines(keep string) {
	cfg := f.app.svc.GetConfig(f.app.ctx())
	f.machines = cfg.Machines
	f.cbMachine.Items().Clear()
	sel := 0
	for i, m := range f.machines {
		f.cbMachine.Items().Add(m.Name)
		if strings.EqualFold(m.Name, keep) {
			sel = i
		}
	}
	if len(f.machines) > 0 {
		f.cbMachine.SetItemIndex(int32(sel))
		f.onSourceChanged()
	} else {
		f.rows, f.view = nil, nil
		f.fillList()
		f.updateStatusPanels()
	}
}

func (f *TMainForm) onSourceChanged() {
	f.updateSourceLabel()
	f.reload(false)
}

func (f *TMainForm) updateSourceLabel() {
	if m := f.curMachine(); m != "" {
		f.lblSourceIs.SetCaption(f.T("lbl_source_is", m+" / "+f.curNode()))
	} else {
		f.lblSourceIs.SetCaption("")
	}
}

// reload reads the models and the allowed targets of the chosen node (off the UI thread: the
// share may be slow).
func (f *TMainForm) reload(keepStatus bool) {
	m, node := f.curMachine(), f.curNode()
	if m == "" {
		return
	}
	f.loadSeq++
	seq := f.loadSeq
	if !keepStatus {
		f.setStatus("st_reading", m, node)
	}
	ctx := f.app.ctx()
	go func() {
		list, err := f.app.svc.ListModels(ctx, m, node)
		var tg service.TargetList
		if err == nil {
			tg, err = f.app.svc.Targets(ctx, m, node)
		}
		quick(func() {
			if seq != f.loadSeq {
				return
			}
			if err != nil {
				f.rows, f.view, f.repository = nil, nil, ""
				f.fillList()
				f.setStatus("st_read_fail", m, node, err.Error())
				f.updateStatusPanels()
				return
			}
			f.rows = list.Models
			f.repository = list.Repository
			f.readOnly = list.ReadOnly
			f.singles, f.groups = tg.Single, tg.Group
			f.fillTargets(m, node)
			f.applyFilter()
			if !keepStatus {
				f.setStatus("st_read_ok", m, node, len(f.rows))
			}
			f.updateStatusPanels()
		})
	}()
}

func (f *TMainForm) fillTargets(m, node string) {
	f.cbSingle.Items().Clear()
	pick := 0
	for i, t := range f.singles {
		f.cbSingle.Items().Add(t.Machine + " / " + string(t.Node))
		if t.Machine == m && string(t.Node) != node && pick == 0 {
			pick = i
		}
	}
	if len(f.singles) > 0 {
		f.cbSingle.SetItemIndex(int32(pick))
	}
	f.cbGroup.Items().Clear()
	for _, g := range f.groups {
		f.cbGroup.Items().Add(g)
	}
	if len(f.groups) > 0 {
		f.cbGroup.SetItemIndex(0)
	} else {
		f.rbSingle.SetChecked(true)
	}
	f.updateEnabled()
}

func (f *TMainForm) yesNo(b bool) string {
	if b {
		return f.T("yes_mark")
	}
	return f.T("no_mark")
}

func (f *TMainForm) cells(r service.ModelRow) []string {
	insp := f.T("no_mark")
	if r.Inspect > 0 {
		insp = strconv.Itoa(r.Inspect)
	}
	mod := ""
	if t, err := time.Parse(time.RFC3339, r.ModTime); err == nil {
		mod = t.Local().Format("2006-01-02 15:04")
	}
	return []string{r.Class, f.yesNo(r.Repo), f.yesNo(r.History), insp, f.yesNo(r.Light), mod}
}

func (f *TMainForm) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(f.edSearch.Text()))
	f.view = f.view[:0:0]
	for _, r := range f.rows {
		if q == "" || strings.Contains(strings.ToLower(r.Name), q) {
			f.view = append(f.view, r)
		}
	}
	f.sortView()
	f.fillList()
	f.updateStatusPanels()
}

func (f *TMainForm) sortBy(col int) {
	if col == f.sortCol {
		f.sortDesc = !f.sortDesc
	} else {
		f.sortCol, f.sortDesc = col, false
	}
	f.sortView()
	f.fillList()
}

func (f *TMainForm) sortView() {
	key := func(r service.ModelRow) string {
		switch f.sortCol {
		case 0:
			return strings.ToLower(r.Name)
		case 4:
			return strconv.Itoa(1000 + r.Inspect)
		case 6:
			return r.ModTime
		}
		return strings.ToLower(f.cells(r)[f.sortCol-1])
	}
	sort.SliceStable(f.view, func(i, j int) bool {
		a, b := key(f.view[i]), key(f.view[j])
		if f.sortDesc {
			return a > b
		}
		return a < b
	})
}

// fillList rebuilds the rows and keeps the selected model selected.
func (f *TMainForm) fillList() {
	f.populating = true
	f.lv.Items().BeginUpdate()
	f.lv.Items().Clear()
	var keep *lcl.TListItem
	for _, r := range f.view {
		it := f.lv.Items().Add()
		it.SetCaption(r.Name)
		for _, c := range f.cells(r) {
			it.SubItems().Add(c)
		}
		if strings.EqualFold(r.Name, f.selected) {
			keep = it
		}
	}
	f.lv.Items().EndUpdate()
	if keep != nil {
		f.lv.SetSelected(keep)
	}
	f.populating = false
	f.onSelect()
}

func (f *TMainForm) onSelect() {
	if f.populating {
		return
	}
	f.selected = ""
	if it := f.lv.Selected(); it != nil {
		f.selected = it.Caption()
	}
	f.updateEnabled()
}

func (f *TMainForm) updateEnabled() {
	has := f.selected != "" && !f.busy
	grp := f.rbGroup.Checked()
	f.rbSingle.SetEnabled(len(f.singles) > 0)
	f.rbGroup.SetEnabled(len(f.groups) > 0)
	f.cbSingle.SetEnabled(!grp && len(f.singles) > 0)
	f.cbGroup.SetEnabled(grp && len(f.groups) > 0)
	ok := has && ((grp && f.cbGroup.ItemIndex() >= 0) || (!grp && f.cbSingle.ItemIndex() >= 0))
	f.btnCopy.SetEnabled(ok)
	f.btnRename.SetEnabled(has)
}

// ---- operations ---------------------------------------------------------------

func (f *TMainForm) doCopy() {
	m := f.curMachine()
	if m == "" || f.selected == "" {
		return
	}
	src := policy.Ref{Machine: m, Node: machine.NodeKind(f.curNode())}
	req := ops.CopyRequest{Source: src, Model: f.selected, NewName: strings.TrimSpace(f.edCopyName.Text())}
	if f.rbGroup.Checked() {
		i := int(f.cbGroup.ItemIndex())
		if i < 0 || i >= len(f.groups) {
			return
		}
		req.Mode = "group"
		req.Target = policy.Ref{Machine: f.groups[i], Node: src.Node}
	} else {
		i := int(f.cbSingle.ItemIndex())
		if i < 0 || i >= len(f.singles) {
			return
		}
		req.Mode = "single"
		req.Target = f.singles[i]
	}
	ctx := f.app.ctx()
	f.runPlan(func() (*ops.Plan, error) { return f.app.svc.PlanCopy(ctx, req) })
}

func (f *TMainForm) doRename() {
	m := f.curMachine()
	if m == "" || f.selected == "" {
		return
	}
	mode := "single"
	if f.chkRenameGroup.Checked() {
		mode = "group"
	}
	req := ops.RenameRequest{Mode: mode, Ref: policy.Ref{Machine: m, Node: machine.NodeKind(f.curNode())},
		Model: f.selected, NewName: strings.TrimSpace(f.edRenameName.Text())}
	ctx := f.app.ctx()
	f.runPlan(func() (*ops.Plan, error) { return f.app.svc.PlanRename(ctx, req) })
}

// runPlan: plan -> confirm -> run -> refresh. Nothing changes before the user confirms.
func (f *TMainForm) runPlan(plan func() (*ops.Plan, error)) {
	f.busy = true
	f.updateEnabled()
	f.setStatus("st_planning")
	finish := func() { f.busy = false; f.updateEnabled() }
	go func() {
		p, err := plan()
		f.app.inUI(func() {
			if err != nil {
				f.setStatus("st_cannot", err.Error())
				f.app.showMessage(f.T("dlg_cannot")+"\n\n"+err.Error(), true)
				finish()
				return
			}
			if !showPlan(f.app, f.TForm, p) {
				f.setStatus("st_canceled")
				finish()
				return
			}
			job, err := f.app.svc.StartJob(f.app.ctx(), p.ID)
			if err != nil {
				f.app.showMessage(f.T("dlg_cannot_start")+"\n\n"+err.Error(), true)
				finish()
				return
			}
			f.setStatusText(p.Title + " ...")
			done := showJob(f.app, f.TForm, job)
			switch done.State {
			case "done":
				f.setStatus("st_done", p.Title)
			case "canceled":
				f.setStatus("st_job_canceled", p.Title)
			default:
				msg := done.Error
				if msg == "" {
					msg = p.Title
				}
				f.setStatus("st_failed", msg)
			}
			finish()
			f.reload(true)
		})
	}()
}

func (f *TMainForm) editMachines() {
	keep := f.curMachine()
	if showMachines(f.app, f.TForm) {
		f.loadMachines(keep)
	}
}

func (f *TMainForm) exportExcel() {
	err := f.app.svc.ExportExcel(f.app.ctx())
	if err == nil {
		return
	}
	if service.Status(err) == 501 {
		f.app.showMessage(err.Error()+"\n"+f.T("export_note"), false)
		return
	}
	f.app.showMessage(err.Error(), true)
}

func (f *TMainForm) about() {
	f.app.showMessage(f.T("about_text", f.app.svc.Version), false)
}
