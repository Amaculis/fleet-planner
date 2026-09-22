package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/buscompany/bus_fleet/internal/config"
)

func validEnv() map[string]string {
	return map[string]string{
		"APP_ENV":      "production",
		"APP_BASE_URL": "https://fleet.example.lv",
		"DATABASE_URL": "postgres://fleet_app:pw@db:5432/fleet",
		"APP_SECRET":   strings.Repeat("ab", 32), // 32 bytes, hex
	}
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoadValid(t *testing.T) {
	setEnv(t, validEnv())
	t.Setenv("SESSION_IDLE_TIMEOUT", "30m")
	t.Setenv("SESSION_ABSOLUTE_TIMEOUT", "8h")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.IsProduction() {
		t.Fatal("IsProduction() = false for APP_ENV=production")
	}
	if cfg.SessionIdleTimeout != 30*time.Minute || cfg.SessionAbsoluteTimeout != 8*time.Hour {
		t.Fatalf("timeouts = %v / %v", cfg.SessionIdleTimeout, cfg.SessionAbsoluteTimeout)
	}
	if len(cfg.AppSecret) != 32 {
		t.Fatalf("AppSecret is %d bytes, want 32", len(cfg.AppSecret))
	}
	// Trusting proxy headers must be opt-in.
	if cfg.TrustProxyHeaders {
		t.Fatal("TrustProxyHeaders defaults to true; it must default to false")
	}
}

// Every one of these must fail startup rather than run with a weak default.
func TestLoadRejectsBadConfig(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]string)
		wantMsg string
	}{
		{name: "missing database url", mutate: func(e map[string]string) { delete(e, "DATABASE_URL") }, wantMsg: "DATABASE_URL"},
		{name: "missing base url", mutate: func(e map[string]string) { delete(e, "APP_BASE_URL") }, wantMsg: "APP_BASE_URL"},
		{name: "plain http in production", mutate: func(e map[string]string) { e["APP_BASE_URL"] = "http://fleet.example.lv" }, wantMsg: "https"},
		{name: "relative base url", mutate: func(e map[string]string) { e["APP_BASE_URL"] = "/fleet" }, wantMsg: "APP_BASE_URL"},
		{name: "missing app secret", mutate: func(e map[string]string) { delete(e, "APP_SECRET") }, wantMsg: "APP_SECRET"},
		{name: "short app secret", mutate: func(e map[string]string) { e["APP_SECRET"] = "abcd" }, wantMsg: "APP_SECRET"},
		{name: "non-hex app secret", mutate: func(e map[string]string) { e["APP_SECRET"] = strings.Repeat("zz", 32) }, wantMsg: "APP_SECRET"},
		{name: "unknown environment", mutate: func(e map[string]string) { e["APP_ENV"] = "staging" }, wantMsg: "APP_ENV"},
		{name: "bad duration", mutate: func(e map[string]string) { e["SESSION_IDLE_TIMEOUT"] = "soon" }, wantMsg: "SESSION_IDLE_TIMEOUT"},
		{name: "idle longer than absolute", mutate: func(e map[string]string) {
			e["SESSION_IDLE_TIMEOUT"] = "24h"
			e["SESSION_ABSOLUTE_TIMEOUT"] = "1h"
		}, wantMsg: "SESSION_IDLE_TIMEOUT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := validEnv()
			// Clear the optional vars so one case cannot leak into another.
			t.Setenv("SESSION_IDLE_TIMEOUT", "")
			t.Setenv("SESSION_ABSOLUTE_TIMEOUT", "")
			tt.mutate(env)
			setEnv(t, env)
			// A deleted key must be empty in the process environment too.
			for _, key := range []string{"DATABASE_URL", "APP_BASE_URL", "APP_SECRET"} {
				if _, ok := env[key]; !ok {
					t.Setenv(key, "")
				}
			}

			_, err := config.Load()
			if err == nil {
				t.Fatal("expected startup to fail")
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("error %q does not mention %q", err, tt.wantMsg)
			}
			// Config errors go to logs: they must name variables, never values.
			if strings.Contains(err.Error(), "fleet_app:pw") {
				t.Fatal("the error leaked a credential")
			}
		})
	}
}
