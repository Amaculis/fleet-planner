// Package config loads all runtime configuration from the environment (12-factor).
// It fails fast: a missing or malformed required variable is a startup error, never a
// silent default.
package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env         string // "production" or "development"
	ListenAddr  string
	BaseURL     string
	DatabaseURL string

	// AppSecret keys the HMAC that protects the pre-login (anonymous) CSRF token.
	// 32 random bytes, hex-encoded in the environment.
	AppSecret []byte

	SessionIdleTimeout     time.Duration
	SessionAbsoluteTimeout time.Duration

	// TrustProxyHeaders must only be true when the app is reachable exclusively through
	// a reverse proxy that overwrites X-Forwarded-For (our Caddy does). If it were true
	// on a directly reachable app, any client could spoof its IP past the rate limiter.
	TrustProxyHeaders bool

	LogLevel string

	// Rate limits, per client IP. The login bucket is deliberately strict; the global
	// one only stops a flood. Both are tunable so a busy office does not trip them.
	LoginRateBurst  int
	LoginRateEvery  time.Duration
	GlobalRateBurst int
	GlobalRateEvery time.Duration

	// Location is the zone the UI displays and parses times in. Storage is always UTC;
	// this only affects the view boundary. The binary embeds the zone database
	// (time/tzdata in main), so the distroless image needs no tzdata package.
	TimeZone string
	Location *time.Location
}

// IsProduction reports whether cookies must carry the Secure attribute and error
// details must stay out of responses.
func (c Config) IsProduction() bool { return c.Env == "production" }

func Load() (Config, error) {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	cfg := Config{
		Env:        envOr("APP_ENV", "development"),
		ListenAddr: envOr("LISTEN_ADDR", ":8080"),
		LogLevel:   envOr("LOG_LEVEL", "info"),
	}

	if cfg.Env != "production" && cfg.Env != "development" {
		fail("APP_ENV must be 'production' or 'development', got %q", cfg.Env)
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		fail("DATABASE_URL is required")
	}

	cfg.BaseURL = os.Getenv("APP_BASE_URL")
	if cfg.BaseURL == "" {
		fail("APP_BASE_URL is required")
	} else if u, err := url.Parse(cfg.BaseURL); err != nil || u.Host == "" {
		fail("APP_BASE_URL must be an absolute URL, got %q", cfg.BaseURL)
	} else if cfg.Env == "production" && u.Scheme != "https" {
		fail("APP_BASE_URL must be https in production")
	}

	secret := os.Getenv("APP_SECRET")
	switch {
	case secret == "":
		fail("APP_SECRET is required (generate with: openssl rand -hex 32)")
	default:
		raw, err := hex.DecodeString(secret)
		if err != nil || len(raw) < 32 {
			fail("APP_SECRET must be at least 32 bytes, hex-encoded")
		} else {
			cfg.AppSecret = raw
		}
	}

	var err error
	if cfg.SessionIdleTimeout, err = durationOr("SESSION_IDLE_TIMEOUT", 4*time.Hour); err != nil {
		errs = append(errs, err)
	}
	if cfg.SessionAbsoluteTimeout, err = durationOr("SESSION_ABSOLUTE_TIMEOUT", 12*time.Hour); err != nil {
		errs = append(errs, err)
	}
	if cfg.SessionIdleTimeout > cfg.SessionAbsoluteTimeout {
		fail("SESSION_IDLE_TIMEOUT must not exceed SESSION_ABSOLUTE_TIMEOUT")
	}

	if cfg.TrustProxyHeaders, err = boolOr("TRUST_PROXY_HEADERS", false); err != nil {
		errs = append(errs, err)
	}

	if cfg.LoginRateBurst, err = intOr("LOGIN_RATE_BURST", 5); err != nil {
		errs = append(errs, err)
	}
	if cfg.LoginRateEvery, err = durationOr("LOGIN_RATE_EVERY", time.Minute); err != nil {
		errs = append(errs, err)
	}
	if cfg.GlobalRateBurst, err = intOr("GLOBAL_RATE_BURST", 120); err != nil {
		errs = append(errs, err)
	}
	if cfg.GlobalRateEvery, err = durationOr("GLOBAL_RATE_EVERY", 500*time.Millisecond); err != nil {
		errs = append(errs, err)
	}

	cfg.TimeZone = envOr("APP_TIMEZONE", "Europe/Riga")
	if cfg.Location, err = time.LoadLocation(cfg.TimeZone); err != nil {
		fail("APP_TIMEZONE is not a known IANA time zone (e.g. Europe/Riga)")
	}

	if len(errs) > 0 {
		// Only variable names are reported, never their values: this output reaches logs.
		msg := "invalid configuration:"
		for _, e := range errs {
			msg += "\n  - " + e.Error()
		}
		return Config{}, fmt.Errorf("%s", msg)
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationOr(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration (e.g. 30m, 12h)", key)
	}
	return d, nil
}

func intOr(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return n, nil
}

func boolOr(key string, fallback bool) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return b, nil
}
