package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("LOADOUT_ADDR", "")
	t.Setenv("LOADOUT_WEB_DIR", "")
	cfg, err := Load()
	if err != nil || cfg.Addr != "127.0.0.1:8787" || cfg.WebDir != "web/dist" {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	t.Setenv("LOADOUT_ADDR", "0.0.0.0:9000")
	t.Setenv("LOADOUT_WEB_DIR", "/tmp/loadout-web")
	cfg, err = Load()
	if err != nil || cfg.Addr != "0.0.0.0:9000" || cfg.WebDir != "/tmp/loadout-web" {
		t.Fatalf("overrides: %+v %v", cfg, err)
	}
}

func TestRejectInvalidAddress(t *testing.T) {
	for _, addr := range []string{"localhost", "127.0.0.1:abc", "127.0.0.1:70000", "127.0.0.1:-1"} {
		t.Run(addr, func(t *testing.T) {
			t.Setenv("LOADOUT_ADDR", addr)
			if _, err := Load(); err == nil {
				t.Fatal("invalid address accepted")
			}
		})
	}
}
