// Command afvi-lcl is the Lazarus LCL version of the GUI (Go binding energye/golcl + the prebuilt liblcl
// native library). The service layer is called in process; the look is chosen with -skin.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/service"
	"afvi/tools/core/internal/wailsapp"

	"github.com/energye/golcl/energy/inits"
	"github.com/energye/golcl/lcl"
)

var (
	version  = "dev"
	theApp   *App
	mainForm *TMainForm
	theSelf  *selftestRun
)

func main() {
	cfgPath := flag.String("config", "", "configuration file (default: <user config dir>/AFVI_TOOLS/config.json)")
	langFlag := flag.String("lang", "", "ko, zh-Hans, zh-Hant or en (default: the saved choice, else ko)")
	skinFlag := flag.String("skin", "", "win2000, platinum or native (default: platinum on macOS, win2000 elsewhere)")
	fontPt := flag.Int("fontsize", 0, "font size in points (default: the toolkit's)")
	selftest := flag.String("selftest", "", "run the scripted self-test with throw-away machines in this directory")
	flag.Parse()

	skin := *skinFlag
	if skin == "" {
		skin = "win2000"
		if runtime.GOOS == "darwin" {
			skin = "platinum"
		}
	}
	pickSkin(skin)

	var store *machine.Store
	var self *wailsapp.Selftest
	var err error
	if *selftest != "" {
		self, store, err = wailsapp.NewSelftest(*selftest)
	} else {
		p := *cfgPath
		if p == "" {
			if p, err = machine.DefaultPath(); err != nil {
				log.Fatal(err)
			}
		}
		store, _, err = machine.Open(filepath.Clean(p))
	}
	if err != nil {
		log.Fatal(err)
	}
	svc := service.New(version, store, false)
	theApp = &App{svc: svc, loc: newLoc(), fontPt: int32(*fontPt)}
	if *selftest != "" {
		theApp.autoConfirm = true
		theSelf = &selftestRun{wapp: wailsapp.New(svc, self)}
	} else {
		theApp.loc.Set(loadLanguage())
	}
	if *langFlag != "" {
		theApp.loc.Set(*langFlag)
	}

	lcl.DEBUG = os.Getenv("AFVI_LCL_DEBUG") != "" // let panics in event handlers show their stack
	inits.Init(nil, nil)
	lcl.RunApp(&mainForm)
	if theSelf != nil {
		os.Exit(theSelf.exitCode())
	}
	fmt.Print()
}
