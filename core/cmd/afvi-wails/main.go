//go:build desktop

// Command afvi-wails is the Wails (system webview) version of the GUI: the service layer is
// bound straight to the front end, in the same process. Build it with the wails CLI, which
// supplies the "desktop" tag:
//
//	wails build -platform windows/amd64 -skipbindings -trimpath
//	wails build -platform linux/amd64 -tags webkit2_41 -skipbindings -trimpath
package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/service"
	"afvi/tools/core/internal/wailsapp"
)

//go:embed all:frontend/dist
var assets embed.FS

var version = "dev"

func main() {
	cfgPath := flag.String("config", "", "configuration file (default: <user config dir>/AFVI_TOOLS/config.json)")
	allowWrite := flag.Bool("allow-network-write", false, "lift the dev-phase rule that network paths are read-only")
	selftest := flag.String("selftest", "", "run the scripted UI self-test with throw-away machines in this directory")
	nativeFrame := flag.Bool("native-frame", false, "use the operating system's title bar instead of the drawn Windows 2000 caption")
	flag.Parse()

	var self *wailsapp.Selftest
	var store *machine.Store
	var err error
	if *selftest != "" {
		self, store, err = wailsapp.NewSelftest(*selftest)
	} else {
		path := *cfgPath
		if path == "" {
			if path, err = machine.DefaultPath(); err != nil {
				log.Fatal(err)
			}
		}
		store, _, err = machine.Open(filepath.Clean(path))
	}
	if err != nil {
		log.Fatal(err)
	}

	app := wailsapp.New(service.New(version, store, *allowWrite), self)
	err = wails.Run(&options.App{
		Title:            "AFVI TOOLS",
		Width:            1100,
		Height:           720,
		MinWidth:         900,
		MinHeight:        560,
		Frameless:        !*nativeFrame, // the caption bar is drawn by the page
		BackgroundColour: &options.RGBA{R: 212, G: 208, B: 200, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		Bind:             []interface{}{app},
		OnStartup: func(ctx context.Context) {
			app.Quit = func() { wruntime.Quit(ctx) }
			if self != nil {
				go func() { os.Exit(<-self.Done) }()
			}
		},
		Windows: &windows.Options{DisableWindowIcon: false, Theme: windows.Light},
		Linux:   &linux.Options{WebviewGpuPolicy: linux.WebviewGpuPolicyNever, ProgramName: "afvi-tools"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
