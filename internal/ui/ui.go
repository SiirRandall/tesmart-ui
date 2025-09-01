package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/SiirRandall/tesmart-ui/internal/client"
	"github.com/SiirRandall/tesmart-ui/internal/config"
	"github.com/SiirRandall/tesmart-ui/internal/widgets"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type AppUI struct {
	cfg          *config.Config
	cli          *client.Client
	app          fyne.App
	win          fyne.Window
	status       *widget.Label
	tiles        map[int]*widgets.PortTile
	ticker       *time.Ticker
	doneCh       chan struct{}
	pendingMu    sync.Mutex
	pendingPort  int
	pendingUntil time.Time
}

func NewAppUI(cfg *config.Config, cli *client.Client) *AppUI {
	return &AppUI{
		cfg:   cfg,
		cli:   cli,
		app:   app.New(),
		tiles: map[int]*widgets.PortTile{},
	}
}

func (u *AppUI) Run() {
	u.win = u.app.NewWindow("TeSmart 16-Port HDMI Switch")
	u.win.Resize(fyne.NewSize(700, 680))

	gridWrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(170, 140)))

	for i := 1; i <= 16; i++ {
		meta := u.cfg.Ports[i]
		iconRes := loadIcon(u.cfg.Dir(), meta.Icon)
		port := i
		t := widgets.NewPortTile(port, meta.Name, iconRes, func() {
			u.beginPending(port, time.Duration(u.cfg.SwitchSuppressMs)*time.Millisecond)
			u.setActiveHighlight(port)
			go u.switchTo(port)
		})
		u.tiles[i] = t
		gridWrap.Add(container.NewPadded(t))
	}

	u.status = widget.NewLabel(fmt.Sprintf("Connected to %s:%d", u.cfg.IP, u.cfg.Port))

	u.win.SetMainMenu(u.buildMenu())
	u.win.SetContent(container.NewBorder(u.buildToolbar(), u.status, nil, nil, gridWrap))
	u.win.SetOnClosed(func() { u.stopPoller() })

	u.startPoller(u.cfg.PollIntervalMs)
	u.win.ShowAndRun()
}

func (u *AppUI) buildMenu() *fyne.MainMenu {
	pingItem := fyne.NewMenuItem("Ping", func() { go u.doPing() })

	buzzerItem := fyne.NewMenuItem("Buzzer", nil)
	buzzerItem.ChildMenu = fyne.NewMenu("",
		fyne.NewMenuItem("Mute", func() { go u.doBuzzer(false) }),
		fyne.NewMenuItem("Unmute", func() { go u.doBuzzer(true) }),
	)

	ledItem := fyne.NewMenuItem("LED Timeout", nil)
	ledItem.ChildMenu = fyne.NewMenu("",
		fyne.NewMenuItem("Off (always on)", func() { go u.doTimeout("off") }),
		fyne.NewMenuItem("10 seconds", func() { go u.doTimeout("10s") }),
		fyne.NewMenuItem("30 seconds", func() { go u.doTimeout("30s") }),
	)

	rawItem := fyne.NewMenuItem("Send Raw Hex…", func() { u.showRawDialog() })
	netCfgItem := fyne.NewMenuItem("Network Config…", func() { u.showNetworkConfigDialog() })

	deviceMenu := fyne.NewMenu("Device",
		pingItem,
		fyne.NewMenuItemSeparator(),
		buzzerItem,
		ledItem,
		fyne.NewMenuItemSeparator(),
		netCfgItem,
		fyne.NewMenuItemSeparator(),
		rawItem,
	)

	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Connection…", func() { u.showConnectionDialog() }),
		fyne.NewMenuItem("Edit Names / Icons…", func() { u.showEditDialog() }),
		fyne.NewMenuItem("Open Config Folder…", func() { openFolder(u.cfg.Dir()) }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() { u.app.Quit() }),
	)

	helpMenu := fyne.NewMenu("Help", fyne.NewMenuItem("About", func() { u.showAbout() }))
	return fyne.NewMainMenu(fileMenu, deviceMenu, helpMenu)
}

func (u *AppUI) buildToolbar() *widget.Toolbar {
	return widget.NewToolbar(
		widget.NewToolbarAction(theme.SettingsIcon(), func() { u.showConnectionDialog() }),
		widget.NewToolbarAction(theme.DocumentIcon(), func() { u.showEditDialog() }),
		widget.NewToolbarAction(theme.ComputerIcon(), func() { u.showNetworkConfigDialog() }),
		widget.NewToolbarAction(theme.MediaPlayIcon(), func() { go u.doPing() }),
		widget.NewToolbarAction(theme.InfoIcon(), func() { u.showAbout() }),
	)
}

/* Polling logic */

func (u *AppUI) startPoller(intervalMs int) {
	u.stopPoller()
	u.ticker = time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	u.doneCh = make(chan struct{})
	go func() {
		u.pollOnce()
		for {
			select {
			case <-u.ticker.C:
				u.pollOnce()
			case <-u.doneCh:
				return
			}
		}
	}()
}

func (u *AppUI) stopPoller() {
	if u.ticker != nil {
		u.ticker.Stop()
	}
	if u.doneCh != nil {
		close(u.doneCh)
	}
}

func (u *AppUI) pollOnce() {
	port, err := u.cli.GetActiveInput()
	if err != nil {
		fyne.Do(func() { u.status.SetText("Polling error: " + err.Error()) })
		return
	}
	if u.shouldIgnore(port) {
		return
	}
	u.clearPendingIfMatch(port)
	fyne.Do(func() {
		u.setActiveHighlight(port)
		u.status.SetText(fmt.Sprintf("Active: %d", port))
	})
}

func (u *AppUI) switchTo(port int) {
	if err := u.cli.SetInput(port); err != nil {
		fyne.Do(func() {
			u.status.SetText("Switch failed")
			dialog := widget.NewLabel(err.Error())
			_ = dialog // you can show a dialog if desired
		})
		u.beginPending(0, 0)
		return
	}
	if u.cfg.FastMode {
		fyne.Do(func() { u.status.SetText(fmt.Sprintf("Switched (fast) → %d", port)) })
		return
	}
	if u.cfg.VerifyAfterSet {
		ok := false
		for attempt := 0; attempt < 2; attempt++ {
			time.Sleep(90 * time.Millisecond)
			if cur, err := u.cli.GetActiveInput(); err == nil && cur == port {
				ok = true
				break
			}
		}
		fyne.Do(func() {
			if ok {
				u.clearPendingIfMatch(port)
				u.status.SetText(fmt.Sprintf("Switched to input %d", port))
			} else {
				u.status.SetText("Switched (unverified) — will sync on next poll")
			}
		})
		return
	}
	fyne.Do(func() { u.status.SetText(fmt.Sprintf("Switched to input %d", port)) })
}

/* Highlight + pending window */

func (u *AppUI) setActiveHighlight(n int) {
	for i, t := range u.tiles {
		t.SetSelected(i == n)
	}
}
func (u *AppUI) beginPending(port int, dur time.Duration) {
	u.pendingMu.Lock()
	u.pendingPort = port
	u.pendingUntil = time.Now().Add(dur)
	u.pendingMu.Unlock()
}
func (u *AppUI) shouldIgnore(polled int) bool {
	u.pendingMu.Lock()
	defer u.pendingMu.Unlock()
	if time.Now().After(u.pendingUntil) {
		return false
	}
	return polled != u.pendingPort
}
func (u *AppUI) clearPendingIfMatch(polled int) {
	u.pendingMu.Lock()
	if polled == u.pendingPort {
		u.pendingUntil = time.Time{}
	}
	u.pendingMu.Unlock()
}

/* Small helpers */

func openFolder(path string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", path).Start()
	case "windows":
		_ = exec.Command("explorer", path).Start()
	default:
		_ = exec.Command("xdg-open", path).Start()
	}
}
