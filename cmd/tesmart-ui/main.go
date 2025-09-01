package main

import (
	"fmt"
	"os"

	"github.com/SiirRandall/tesmart-ui/internal/client"
	"github.com/SiirRandall/tesmart-ui/internal/config"
	"github.com/SiirRandall/tesmart-ui/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Config error:", err)
		os.Exit(1)
	}

	cli := client.New(cfg.IP, cfg.Port,
		cfg.GetTimeout(), cfg.SetTimeout())

	app := ui.NewAppUI(cfg, cli)
	app.Run()
}
