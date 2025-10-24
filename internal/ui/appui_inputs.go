package ui

import (
	"fmt"
	"sort"
)

// InputNames returns input names from config.Ports in ascending port order (1..16).
// Falls back to generic "Port N" names if config is missing.
func (u *AppUI) InputNames() []string {
	// default 16 generic names
	def := func() []string {
		out := make([]string, 0, 16)
		for i := 1; i <= 16; i++ {
			out = append(out, fmt.Sprintf("Port %d", i))
		}
		return out
	}

	if u == nil || u.cfg == nil || u.cfg.Ports == nil || len(u.cfg.Ports) == 0 {
		return def()
	}

	// sort ports numerically so the menu order is stable
	keys := make([]int, 0, len(u.cfg.Ports))
	for k := range u.cfg.Ports {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	names := make([]string, 0, len(keys))
	for _, k := range keys {
		meta := u.cfg.Ports[k]
		if meta.Name != "" {
			names = append(names, meta.Name)
		} else {
			names = append(names, fmt.Sprintf("Port %d", k))
		}
	}
	return names
}

// SwitchInput resolves a tray label -> port and calls the device.
// Priority:
//  1. Exact name match in cfg.Ports -> use that port
//  2. Fallback to index+1 within InputNames() list
func (u *AppUI) SwitchInput(name string) error {
	if u == nil || u.cli == nil {
		return fmt.Errorf("no client available")
	}

	// Try exact name match in configured ports
	if u.cfg != nil && u.cfg.Ports != nil {
		for port, meta := range u.cfg.Ports {
			if meta.Name == name {
				if port < 1 || port > 16 {
					return fmt.Errorf("invalid port in config: %d", port)
				}
				return u.cli.SetInput(port)
			}
		}
	}

	// Fallback: position in ordered names -> 1-based port
	names := u.InputNames()
	for i, n := range names {
		if n == name {
			return u.cli.SetInput(i + 1)
		}
	}
	return fmt.Errorf("unknown input: %s", name)
}
