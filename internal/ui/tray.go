// internal/ui/tray.go
package ui

import (
	_ "embed"
	"fmt"
	"log"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

//go:embed assets/tray.png
var trayPNG []byte

func setTrayIcon(desk desktop.App) {
	if len(trayPNG) > 0 {
		desk.SetSystemTrayIcon(fyne.NewStaticResource("tray.png", trayPNG))
		log.Printf("[tray] icon set from embed (%d bytes)\n", len(trayPNG))
		return
	}
	log.Println("[tray] WARNING: tray.png embed is empty — check path internal/ui/assets/tray.png")
}

// Cache the real main window (not the tray monitor)
var mainWinRef fyne.Window

func setMainWinRef(w fyne.Window) { mainWinRef = w }
func getMainWinRef(app fyne.App) fyne.Window {
	if mainWinRef != nil {
		return mainWinRef
	}
	return findMainWindow(app)
}

// Consider a window to be the real app window (not Fyne's systray monitor)
func isRealAppWindow(w fyne.Window) bool {
	if w == nil {
		return false
	}
	t := strings.ToLower(w.Title())
	// Common Fyne/OS helper names to skip
	if strings.Contains(t, "systray") ||
		strings.Contains(t, "system tray") ||
		strings.Contains(t, "tray") ||
		strings.Contains(t, "status") ||
		strings.Contains(t, "monitor") {
		return false
	}
	// Must have some content to be a real window
	return w.Content() != nil
}

// Find the first non-tray, non-helper window
func findMainWindow(app fyne.App) fyne.Window {
	if app == nil || app.Driver() == nil {
		return nil
	}
	for _, w := range app.Driver().AllWindows() {
		if isRealAppWindow(w) {
			return w
		}
	}
	return nil
}

// Fallback: try to show any real app window(s)
func showAnyRealWindow(app fyne.App) bool {
	if app == nil || app.Driver() == nil {
		return false
	}
	shown := false
	for _, w := range app.Driver().AllWindows() {
		if !isRealAppWindow(w) {
			continue
		}
		w.Show()
		w.RequestFocus()
		w.CenterOnScreen()
		shown = true
	}
	if shown {
		log.Println("[tray] Show Window: displayed real app window(s)")
	}
	return shown
}

// EnableSystemTray installs the tray/menu and makes Close -> Hide on the real window.
func (u *AppUI) EnableSystemTray() {
	app := fyne.CurrentApp()
	if app == nil {
		log.Println("[tray] no CurrentApp (called too early?)")
		return
	}

	// Install tray immediately
	if desk, ok := app.(desktop.App); ok {
		setTrayIcon(desk) // <— use the helper
		desk.SetSystemTrayMenu(u.buildTrayMenu(app))
		log.Println("[tray] system tray menu installed")
	} else {
		log.Println("[tray] desktop.App not available (non-desktop build?)")
	}

	// Attach Close->Hide when the UI loop starts.
	app.Lifecycle().SetOnStarted(func() {
		if w := findMainWindow(app); w != nil {
			setMainWinRef(w)
			w.SetCloseIntercept(func() { w.Hide() })
			log.Printf("[tray] close->hide attached to window: %q (immediate)\n", w.Title())
			return
		}

		// If window is created slightly later, retry briefly.
		const (
			totalWait   = 5 * time.Second
			interval    = 150 * time.Millisecond
			maxAttempts = int(totalWait / interval)
		)
		go func() {
			for i := 0; i < maxAttempts; i++ {
				if w := findMainWindow(app); w != nil {
					setMainWinRef(w)
					w.SetCloseIntercept(func() { w.Hide() })
					log.Printf("[tray] close->hide attached to window: %q (delayed)\n", w.Title())
					return
				}
				time.Sleep(interval)
			}
			log.Println("[tray] no real app window found within timeout; close->hide not attached")
		}()
	})
}

func (u *AppUI) buildTrayMenu(app fyne.App) *fyne.Menu { // Submenu: All Inputs (from config)
	inputNames := u.InputNames()
	items := make([]*fyne.MenuItem, 0, len(inputNames))
	for _, name := range inputNames {
		n := name
		items = append(items, fyne.NewMenuItem(n, func() {
			if err := u.SwitchInput(n); err != nil {
				log.Printf("[tray] switch %q failed: %v\n", n, err)
				app.SendNotification(&fyne.Notification{
					Title:   "Switch Failed",
					Content: fmt.Sprintf("Could not switch to %s: %v", n, err),
				})
				return
			}
			app.SendNotification(&fyne.Notification{
				Title:   "Input Switched",
				Content: fmt.Sprintf("Switched to %s", n),
			})
		}))
	}
	allInputsSub := fyne.NewMenu("All Inputs", items...)
	allInputsItem := fyne.NewMenuItem("All Inputs", nil)
	allInputsItem.ChildMenu = allInputsSub

	showItem := fyne.NewMenuItem("Show Window", func() {
		if w := getMainWinRef(app); isRealAppWindow(w) {
			w.Show()
			w.RequestFocus()
			w.CenterOnScreen()
			return
		}
		if showAnyRealWindow(app) {
			return
		}
		log.Println("[tray] Show Window: no real app window available")
		widget.ShowPopUp(widget.NewLabel("No window available to show."), nil)
	})

	configItem := fyne.NewMenuItem("Config…", func() {
		// hook your settings UI if you like
	})
	quitItem := fyne.NewMenuItem("Quit", func() { app.Quit() })

	return fyne.NewMenu("TeSmart UI",
		showItem,
		fyne.NewMenuItemSeparator(),
		allInputsItem,
		configItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	)
}
