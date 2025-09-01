package config

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type PortMeta struct {
	Name string `yaml:"name"`
	Icon string `yaml:"icon"`
}

type Config struct {
	IP               string           `yaml:"ip"`
	Port             int              `yaml:"port"`
	Ports            map[int]PortMeta `yaml:"ports"`
	PollIntervalMs   int              `yaml:"poll_interval_ms"`
	FastMode         bool             `yaml:"fast_mode"`
	GetTimeoutMs     int              `yaml:"get_timeout_ms"`
	SetTimeoutMs     int              `yaml:"set_timeout_ms"`
	VerifyAfterSet   bool             `yaml:"verify_after_set"`
	SwitchSuppressMs int              `yaml:"switch_suppress_ms"`

	fileDir  string `yaml:"-"`
	filePath string `yaml:"-"`
	mu       sync.Mutex
}

var defaultYAML = []byte(`# TeSmart UI (Go/Fyne) config
ip: "192.168.1.10"
port: 5000

poll_interval_ms: 1000

fast_mode: false
get_timeout_ms: 600
set_timeout_ms: 450
verify_after_set: true
switch_suppress_ms: 800

ports:
  1: { name: "PC 1", icon: "" }
  2: { name: "PC 2", icon: "" }
  3: { name: "PC 3", icon: "" }
  4: { name: "PC 4", icon: "" }
  5: { name: "PC 5", icon: "" }
  6: { name: "PC 6", icon: "" }
  7: { name: "PC 7", icon: "" }
  8: { name: "PC 8", icon: "" }
  9: { name: "PC 9", icon: "" }
  10: { name: "PC 10", icon: "" }
  11: { name: "PC 11", icon: "" }
  12: { name: "PC 12", icon: "" }
  13: { name: "PC 13", icon: "" }
  14: { name: "PC 14", icon: "" }
  15: { name: "PC 15", icon: "" }
  16: { name: "PC 16", icon: "" }
`)

func paths() (dir, file string) {
	base, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	dir = filepath.Join(base, "tesmart-ui")
	file = filepath.Join(dir, "config.yaml")
	return
}

func ensure(dir, file string) error {
	if _, err := os.Stat(file); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, defaultYAML, 0o644)
}

func Load() (*Config, error) {
	dir, file := paths()
	if err := ensure(dir, file); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	if cfg.IP == "" {
		cfg.IP = "192.168.1.10"
	}
	if cfg.Port == 0 {
		cfg.Port = 5000
	}
	if cfg.PollIntervalMs <= 0 {
		cfg.PollIntervalMs = 1000
	}
	if cfg.GetTimeoutMs <= 0 {
		cfg.GetTimeoutMs = 600
	}
	if cfg.SetTimeoutMs <= 0 {
		cfg.SetTimeoutMs = 450
	}
	if cfg.SwitchSuppressMs <= 0 {
		cfg.SwitchSuppressMs = 800
	}
	if cfg.Ports == nil {
		cfg.Ports = map[int]PortMeta{}
	}
	for i := 1; i <= 16; i++ {
		if _, ok := cfg.Ports[i]; !ok {
			cfg.Ports[i] = PortMeta{Name: "Port " + strconv.Itoa(i)}
		}
	}
	cfg.fileDir, cfg.filePath = dir, file
	return &cfg, nil
}

func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	out, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.filePath, out, 0o644)
}

func (c *Config) Dir() string  { return c.fileDir }
func (c *Config) Path() string { return c.filePath }

func (c *Config) GetTimeout() time.Duration { return time.Duration(c.GetTimeoutMs) * time.Millisecond }
func (c *Config) SetTimeout() time.Duration { return time.Duration(c.SetTimeoutMs) * time.Millisecond }
