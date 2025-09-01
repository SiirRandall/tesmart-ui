# TESmart UI

A cross-platform GUI for controlling **TESmart 16-port HDMI/KVM switches**.

Built in **Go** with [Fyne](https://fyne.io/), this app provides a simple graphical interface to switch inputs, configure ports, control buzzer/LED, send raw hex commands, and manage the device’s **network configuration**. MIT licensed.

---

##  Features

- **Visual Grid of Ports**  
  16 rounded tiles (dark theme; active tile highlighted blue).  
  Custom names + icons per port.

- **Polling & Switching**  
  Polls the active input (default every 1s, configurable).  
  One‑click switching with flicker suppression & optional verification reads.

- **Device Controls**  
  - Buzzer: mute / unmute  
  - LED timeout: off / 10s / 30s  
  - Ping: quick health check  
  - Raw hex sender: advanced diagnostics

---

## Getting Started

### Requirements

- Go **1.22+**
- Fyne **v2**
- TESmart HDMI/KVM (default IP `192.168.1.10`, port `5000`)

On Linux, install system deps:

```bash
sudo apt update
sudo apt install -y libgtk-3-dev libgl1-mesa-dev xorg-dev libxrandr-dev libxcursor-dev libxinerama-dev libxi-dev
```

### Build & Run

```bash
git clone https://github.com/SiirRandall/tesmart-ui.git
cd tesmart-ui

go mod tidy
go run ./cmd/tesmart-ui
```

Build a binary:

```bash
go build -o tesmart-ui ./cmd/tesmart-ui
```

---

## Configuration

The app creates a YAML config on first run:

- **macOS**: `~/Library/Application Support/tesmart-ui/config.yaml`  
- **Linux**: `~/.config/tesmart-ui/config.yaml`  
- **Windows**: `%AppData%\tesmart-ui\config.yaml`

Example:

```yaml
ip: "192.168.1.10"
port: 5000

poll_interval_ms: 1000
fast_mode: false
get_timeout_ms: 600
set_timeout_ms: 450
verify_after_set: true
switch_suppress_ms: 800

ports:
  1:  { name: "PC 1", icon: "" }
  2:  { name: "Media Box", icon: "icons/media.png" }
  3:  { name: "Console", icon: "" }
  # ...
  16: { name: "Spare", icon: "" }
```

### Editing & Icons

- **File → Connection…** — set app target IP/Port.  
- **File → Edit Names / Icons…** — per‑port labels & icons.  
- **Device → Network Config…** — read/set switch IP/Mask/Gateway/Port.

Icon paths can be absolute or **relative to the config folder**.  
Tile icon size is set in code (default **84×84**) and can be tweaked later if desired.

---

## Protocol Notes

- Input switching & status use **binary frames** (`AABB 03 .. EE`).  
- Network configuration uses **ASCII** commands (`IP?`, `IP:192.168.1.100;`, etc.).  
- Many models require a **power cycle** for new IP/port to take effect.

---

## Credits

- **Code concepts & inspiration:**  
  [mirceanton/tesmartctl](https://github.com/mirceanton/tesmartctl)
- **API documentation reference (especially ASCII LAN commands):**  
  [Kreeblah/TESmartSwitchAPI-macOS](https://github.com/Kreeblah/TESmartSwitchAPI-macOS)

---

## License

MIT - see [LICENSE](LICENSE).

---

## ⚠️ Disclaimer

This project is not affiliated with TESmart. Use at your own risk. Changing network settings can temporarily disconnect the device until you **power‑cycle** and reconnect to the new IP.
