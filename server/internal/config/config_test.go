package config

import (
	"os"
	"strings"
	"testing"
)

func TestInitialCreditsConfiguration(t *testing.T) {
	isolateConfig(t)
	for _, tc := range []struct {
		raw     string
		want    int64
		invalid bool
	}{{"", 1000, false}, {"0", 0, false}, {"42", 42, false}, {"-1", 0, true}, {"1000000000001", 0, true}, {"1.5", 0, true}} {
		t.Run(tc.raw, func(t *testing.T) {
			t.Setenv("LOADOUT_INITIAL_CREDITS", tc.raw)
			cfg, err := Load()
			if tc.invalid {
				if err == nil {
					t.Fatal("invalid initial credits accepted")
				}
				return
			}
			if err != nil || cfg.InitialCredits != tc.want {
				t.Fatalf("initial credits=%d want %d err=%v", cfg.InitialCredits, tc.want, err)
			}
		})
	}
}

func TestProductConfigurationValidation(t *testing.T) {
	isolateConfig(t)
	for _, pair := range [][2]string{{"LOADOUT_DATABASE_URL", "sqlite:///data.db"}, {"LOADOUT_PUBLIC_URL", "javascript:alert(1)"}, {"LOADOUT_ENCRYPTION_KEY", "short"}, {"LOADOUT_ADMIN_USERNAME", "admin"}, {"LOADOUT_ALLOW_PRIVATE_UPSTREAMS", "maybe"}} {
		t.Run(pair[0], func(t *testing.T) {
			t.Setenv(pair[0], pair[1])
			if _, err := Load(); err == nil {
				t.Fatal("invalid product config accepted")
			}
		})
	}
}

func TestIdentityConfigurationRequiresCompleteCredentials(t *testing.T) {
	isolateConfig(t)
	for _, pair := range [][2]string{{"LOADOUT_GITHUB_CLIENT_ID", "client-only"}, {"LOADOUT_SMTP_ADDRESS", "smtp.example.com:587"}, {"LOADOUT_SMTP_ALLOW_LOCAL_INSECURE", "maybe"}, {"LOADOUT_GITHUB_ORG", "org/../../user"}} {
		t.Run(pair[0], func(t *testing.T) {
			t.Setenv(pair[0], pair[1])
			if _, err := Load(); err == nil {
				t.Fatal("incomplete identity configuration accepted")
			}
		})
	}
}

func TestLoad(t *testing.T) {
	isolateConfig(t)
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
	isolateConfig(t)
	for _, addr := range []string{"localhost", "127.0.0.1:abc", "127.0.0.1:70000", "127.0.0.1:-1"} {
		t.Run(addr, func(t *testing.T) {
			t.Setenv("LOADOUT_ADDR", addr)
			if _, err := Load(); err == nil {
				t.Fatal("invalid address accepted")
			}
		})
	}
}

// Config tests must not inherit the developer's exported service credentials.
func isolateConfig(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "LOADOUT_") {
			t.Setenv(key, "")
		}
	}
}

func TestRedisConfiguration(t *testing.T) {
	isolateConfig(t)
	cfg, err := Load()
	if err != nil || cfg.RedisNamespace != "loadout" || cfg.RedisURL != "" {
		t.Fatalf("redis defaults namespace=%q err=%v", cfg.RedisNamespace, err)
	}
	t.Setenv("LOADOUT_REDIS_URL", "redis://localhost:6380/2")
	t.Setenv("LOADOUT_REDIS_NAMESPACE", "staging_1")
	cfg, err = Load()
	if err != nil || cfg.RedisURL != "redis://localhost:6380/2" || cfg.RedisNamespace != "staging_1" {
		t.Fatalf("redis overrides not parsed: %v", err)
	}
	for _, raw := range []string{"http://name:TOP_SECRET@host:6379", "redis://name:TOP_SECRET@host:bad"} {
		t.Setenv("LOADOUT_REDIS_URL", raw)
		if _, err := Load(); err == nil {
			t.Error("invalid redis URL accepted")
		} else if strings.Contains(err.Error(), "TOP_SECRET") {
			t.Error("redis credentials leaked")
		}
	}
	t.Setenv("LOADOUT_REDIS_URL", "")
	t.Setenv("LOADOUT_REDIS_NAMESPACE", "shared:namespace")
	if _, err := Load(); err == nil {
		t.Error("invalid redis namespace accepted")
	}
}
