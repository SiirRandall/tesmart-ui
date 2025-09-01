package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SiirRandall/tesmart-ui/internal/config"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

/* About + icons */

func (u *AppUI) showAbout() {
	dialog.ShowInformation("About",
		"TeSmart 16-Port GUI (Go/Fyne)\n\n• Input switching, ping, buzzer, LED timeout, raw hex\n• Network Config via ASCII (IP?/PT?/MA?/GW?)\n\nConfig: "+u.cfg.Path(),
		u.win)
}

func loadIcon(cfgDir, rel string) fyne.Resource {
	if rel == "" {
		return nil
	}
	path := rel
	if !filepath.IsAbs(path) {
		path = filepath.Join(cfgDir, rel)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(filepath.Base(path), b)
}

/* Connection */

func (u *AppUI) showConnectionDialog() {
	ipEntry := widget.NewEntry()
	ipEntry.SetPlaceHolder("e.g., 192.168.1.10")
	ipEntry.SetText(u.cfg.IP)

	portEntry := widget.NewEntry()
	portEntry.SetPlaceHolder("5000")
	portEntry.SetText(strconv.Itoa(u.cfg.Port))

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "App → KVM IP", Widget: ipEntry},
			{Text: "App → KVM Port", Widget: portEntry},
		},
		OnSubmit: func() {
			ip := strings.TrimSpace(ipEntry.Text)
			p, err := strconv.Atoi(strings.TrimSpace(portEntry.Text))
			if err != nil || p < 1 || p > 65535 {
				dialog.ShowError(fmt.Errorf("invalid port"), u.win)
				return
			}
			if ip == "" {
				dialog.ShowError(fmt.Errorf("IP address cannot be empty"), u.win)
				return
			}
			u.cfg.IP, u.cfg.Port = ip, p
			if err := u.cfg.Save(); err != nil {
				dialog.ShowError(fmt.Errorf("failed to save config: %v", err), u.win)
				return
			}
			u.cli.SetTarget(u.cfg.IP, u.cfg.Port, u.cfg.GetTimeout(), u.cfg.SetTimeout())
			u.status.SetText(fmt.Sprintf("Connection updated → %s:%d", ip, p))
			go u.pollOnce()
		},
		SubmitText: "Save",
	}
	d := dialog.NewCustom("Connection (Client Target)", "Close", form, u.win)
	d.Resize(fyne.NewSize(520, 340))
	d.Show()
}

/* Edit ports UI */

func (u *AppUI) showEditDialog() {
	portOptions := make([]string, 16)
	for i := 1; i <= 16; i++ {
		portOptions[i-1] = fmt.Sprintf("%d", i)
	}
	portSelect := widget.NewSelect(portOptions, nil)
	portSelect.SetSelected("1")

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Display name")

	iconPathEntry := widget.NewEntry()
	iconPathEntry.SetPlaceHolder("Icon path (abs or relative to config folder)")

	prefill := func() {
		pn, _ := strconv.Atoi(portSelect.Selected)
		meta := u.cfg.Ports[pn]
		nameEntry.SetText(meta.Name)
		iconPathEntry.SetText(meta.Icon)
	}
	portSelect.OnChanged = func(string) { prefill() }
	prefill()

	iconPick := widget.NewButtonWithIcon("Choose Icon…", theme.FolderOpenIcon(), func() {
		fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			if err != nil || r == nil {
				return
			}
			iconPathEntry.SetText(r.URI().Path())
			_ = r.Close()
		}, u.win)
		fd.Resize(fyne.NewSize(700, 500))
		fd.Show()
	})

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Port", Widget: portSelect},
			{Text: "Name", Widget: nameEntry},
			{Text: "Icon", Widget: container.NewBorder(nil, nil, nil, iconPick, iconPathEntry)},
		},
		OnSubmit: func() {
			pn, _ := strconv.Atoi(portSelect.Selected)
			u.cfg.Ports[pn] = config.PortMeta{Name: nameEntry.Text, Icon: iconPathEntry.Text}
			_ = u.cfg.Save()

			iconRes := loadIcon(u.cfg.Dir(), iconPathEntry.Text)
			u.tiles[pn].SetNameIcon(nameEntry.Text, iconRes)
			u.status.SetText(fmt.Sprintf("Updated Port %d", pn))
		},
		SubmitText: "Save",
	}
	d := dialog.NewCustom("Edit Names / Icons", "Close", form, u.win)
	d.Resize(fyne.NewSize(640, 460))
	d.Show()
}

/* Network config */

func (u *AppUI) showNetworkConfigDialog() {
	ipEntry := widget.NewEntry()
	maskEntry := widget.NewEntry()
	gwEntry := widget.NewEntry()
	portEntry := widget.NewEntry()

	ipEntry.SetPlaceHolder("e.g., 192.168.1.130")
	maskEntry.SetPlaceHolder("e.g., 255.255.255.0")
	gwEntry.SetPlaceHolder("e.g., 192.168.1.1")
	portEntry.SetPlaceHolder("e.g., 5000")

	getBtn := widget.NewButton("Get Configuration", func() {
		go func() {
			cfgReply, err := u.cli.GetNetworkConfigASCII()
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, u.win)
					return
				}
				ipEntry.SetText(cfgReply.IP)
				portEntry.SetText(strconv.Itoa(cfgReply.Port))
				maskEntry.SetText(cfgReply.Mask)
				gwEntry.SetText(cfgReply.GW)
				u.status.SetText("Fetched network configuration")
			})
		}()
	})

	setBtn := widget.NewButton("Set Configuration", func() {
		ip := strings.TrimSpace(ipEntry.Text)
		mask := strings.TrimSpace(maskEntry.Text)
		gw := strings.TrimSpace(gwEntry.Text)
		p, err := strconv.Atoi(strings.TrimSpace(portEntry.Text))
		if err != nil || p < 1 || p > 65535 {
			dialog.ShowError(fmt.Errorf("invalid port"), u.win)
			return
		}

		go func() {
			err := u.cli.SetNetworkConfigASCII(ip, p, mask, gw)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(fmt.Errorf("set failed: %v", err), u.win)
					return
				}
				u.status.SetText("Network configuration sent")

				dialog.ShowInformation(
					"Network Configuration Updated",
					fmt.Sprintf(
						"New settings have been sent to the switch:\n\nIP: %s\nNetmask: %s\nGateway: %s\nPort: %d\n\n"+
							"Note: You may need to power-cycle (reboot) the switch for the new IP/port to take effect.",
						ip, mask, gw, p,
					),
					u.win,
				)

				// Update local target so the app knows intended destination.
				u.cfg.IP, u.cfg.Port = ip, p
				if e := u.cfg.Save(); e == nil {
					u.cli.SetTarget(u.cfg.IP, u.cfg.Port, u.cfg.GetTimeout(), u.cfg.SetTimeout())
					u.status.SetText(fmt.Sprintf("Target set to %s:%d (will work after device reboot)", u.cfg.IP, u.cfg.Port))
					go u.pollOnce()
				}
			})
		}()
	})

	formTop := widget.NewForm(
		widget.NewFormItem("KVM IPv4 Address", ipEntry),
		widget.NewFormItem("KVM Netmask", maskEntry),
		widget.NewFormItem("KVM Gateway", gwEntry),
		widget.NewFormItem("KVM Port", portEntry),
	)

	body := container.NewVBox(
		widget.NewLabelWithStyle("KVM Network Configuration (ASCII protocol)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		formTop,
		container.NewHBox(getBtn, setBtn, layout.NewSpacer()),
	)

	d := dialog.NewCustom("Device Network Config", "Close", body, u.win)
	d.Resize(fyne.NewSize(620, 360))
	d.Show()
}

/* Raw dialog */

func (u *AppUI) showRawDialog() {
	in := widget.NewEntry()
	in.SetPlaceHolder("Enter hex frame, e.g. AABB031000EE")
	out := widget.NewMultiLineEntry()
	out.SetPlaceHolder("Reply (hex) will appear here")
	out.Wrapping = fyne.TextWrapWord
	out.Disable()

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Raw Hex", Widget: in},
			{Text: "Reply", Widget: out},
		},
		OnSubmit: func() {
			hexstr := strings.TrimSpace(in.Text)
			go func() {
				reply, err := u.cli.RawHexSend(hexstr, 1200*time.Millisecond)
				fyne.Do(func() {
					if err != nil {
						dialog.ShowError(err, u.win)
						return
					}
					out.SetText(reply)
				})
			}()
		},
		SubmitText: "Send",
	}
	d := dialog.NewCustom("Send Raw Hex", "Close", form, u.win)
	d.Resize(fyne.NewSize(640, 420))
	d.Show()
}

/* Quick device actions */

func (u *AppUI) doPing() {
	start := time.Now()
	err := u.cli.Ping()
	lat := time.Since(start)
	fyne.Do(func() {
		if err != nil {
			dialog.ShowError(fmt.Errorf("ping failed: %v", err), u.win)
			u.status.SetText("Ping failed")
		} else {
			dialog.ShowInformation("Ping", fmt.Sprintf("OK in %d ms", lat.Milliseconds()), u.win)
			u.status.SetText(fmt.Sprintf("Ping OK (%d ms)", lat.Milliseconds()))
		}
	})
}

func (u *AppUI) doBuzzer(on bool) {
	if err := u.cli.SetBuzzer(on); err != nil {
		fyne.Do(func() { dialog.ShowError(fmt.Errorf("buzzer command failed: %v", err), u.win) })
		return
	}
	if on {
		u.status.SetText("Buzzer unmuted")
	} else {
		u.status.SetText("Buzzer muted")
	}
}

func (u *AppUI) doTimeout(mode string) {
	var err error
	switch mode {
	case "off":
		err = u.cli.SetLEDTimeoutOff()
	case "10s":
		err = u.cli.SetLEDTimeout10s()
	case "30s":
		err = u.cli.SetLEDTimeout30s()
	default:
		err = fmt.Errorf("unknown timeout mode")
	}
	if err != nil {
		fyne.Do(func() { dialog.ShowError(fmt.Errorf("LED timeout failed: %v", err), u.win) })
		return
	}
	u.status.SetText("LED timeout: " + mode)
}
