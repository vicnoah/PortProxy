package main

import (
	"log"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	"portproxy/internal/data"
	"portproxy/internal/win"
)

func main() {
	// Signed auto-updates, with a native-UI update window (no webview).
	mygo.Use(native.Plugin)

	// Writing HKLM\SYSTEM\CurrentControlSet\Services\PortProxy needs the
	// same administrator rights the original app asks for in its manifest.
	if !win.IsElevated() {
		_ = win.RelaunchElevated()
		return
	}

	// A previous overwrite install may have left a quit signal behind.
	win.ClearUpdateQuitSignal()

	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	if !win.AcquireSingleInstance() {
		win.FocusExistingWindow()
		return
	}

	ownPath, err := data.DefaultDBPath()
	if err != nil {
		log.Fatal(err)
	}
	store, err := data.OpenStore(ownPath)
	if err != nil {
		log.Fatal(err)
	}

	a, err := newApp(store, osPlatform{})
	if err != nil {
		log.Fatal(err)
	}
	a.refresh()

	mygo.App.WhenReady(func() {
		// MYGO_LOCALE wins over the saved choice for one run.
		if tag := data.LoadLanguagePref(); tag != "" && os.Getenv("MYGO_LOCALE") == "" {
			mygo.App.SetLocale(tag)
		}
		mainWin := mygo.NewWindow(mygo.WindowOptions{
			Title: "PortProxy", Width: max(a.cfg.MainWindowWidth, 1180),
			Height: max(a.cfg.MainWindowHeight, 760), MinWidth: 1080, MinHeight: 620,
			Content: ui.View(a.view),
		})
		a.win = mainWin
		a.update = func(fn func()) { mainWin.Update(fn) }
		a.async = func(fn func()) { go fn() }

		mainWin.OnResize(func() {
			w, h := mainWin.Size()
			a.cfg.MainWindowWidth, a.cfg.MainWindowHeight = w, h
		})
		mainWin.OnClose(func(*mygo.CloseEvent) {
			a.saveConfig()
		})

		mygo.App.OnSecondInstance(func([]string, string) {
			a.update(func() {
				mainWin.Show()
				mainWin.Focus()
			})
		})

		go watchUpdateQuit(a, mainWin)
	})

	// Closing the only window quits the app: there is no tray or resident
	// background process.
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// watchUpdateQuit lets the non-elevated NSIS installer close this elevated
// process before it overwrites the executable during an in-place upgrade.
func watchUpdateQuit(a *app, mainWin *mygo.Window) {
	for {
		time.Sleep(150 * time.Millisecond)
		if mainWin.IsDestroyed() || !win.UpdateQuitSignalSet() {
			continue
		}
		win.ClearUpdateQuitSignal()
		mainWin.Update(func() {
			a.saveConfig()
			mygo.App.Quit()
		})
		return
	}
}
