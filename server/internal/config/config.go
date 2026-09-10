package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

type Config struct {
	Addr   string
	WebDir string
}

func Load() (Config, error) {
	cfg := Config{Addr: os.Getenv("LOADOUT_ADDR"), WebDir: os.Getenv("LOADOUT_WEB_DIR")}
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8787"
	}
	if cfg.WebDir == "" {
		cfg.WebDir = "web/dist"
	}
	_, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return Config{}, fmt.Errorf("LOADOUT_ADDR must be host:port")
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 0 || value > 65535 {
		return Config{}, fmt.Errorf("LOADOUT_ADDR port must be between 0 and 65535")
	}
	return cfg, nil
}
