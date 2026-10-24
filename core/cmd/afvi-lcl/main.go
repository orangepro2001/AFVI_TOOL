package main

import (
	"github.com/energye/golcl/energy/inits"
	"github.com/energye/golcl/lcl"
)

type TMainForm struct {
	*lcl.TForm
	btn *lcl.TButton
}

var mainForm *TMainForm

func main() {
	inits.Init(nil, nil)
	lcl.RunApp(&mainForm)
}

func (f *TMainForm) OnFormCreate(sender lcl.IObject) {
	f.SetCaption("hello")
	f.SetWidth(400)
	f.SetHeight(200)
	f.btn = lcl.NewButton(f)
	f.btn.SetParent(f)
	f.btn.SetCaption("Push")
	f.btn.SetLeft(20)
	f.btn.SetTop(20)
	t := lcl.NewTimer(f)
	t.SetInterval(1500)
	t.SetOnTimer(func(lcl.IObject) { f.Close() })
}
