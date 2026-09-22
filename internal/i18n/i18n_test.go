package i18n_test

import (
	"strings"
	"testing"

	"github.com/buscompany/bus_fleet/internal/i18n"
)

// New() fails when a locale is missing keys or carries unknown ones, so this single
// call is the guard against a half-translated UI shipping.
func TestBundleLoadsAndIsComplete(t *testing.T) {
	b, err := i18n.New()
	if err != nil {
		t.Fatalf("loading translations: %v", err)
	}
	if got := b.Supported(); len(got) != 3 || got[0] != "en" {
		t.Fatalf("Supported() = %v, want [en lv ru]", got)
	}
}

func TestMatch(t *testing.T) {
	b, err := i18n.New()
	if err != nil {
		t.Fatalf("loading translations: %v", err)
	}
	lv, ru, unknown := "lv", "ru", "de"

	tests := []struct {
		name           string
		userPref       *string
		acceptLanguage string
		want           string
	}{
		{name: "no preference, no header", want: "en"},
		{name: "user preference wins over header", userPref: &lv, acceptLanguage: "ru-RU,ru;q=0.9", want: "lv"},
		{name: "user preference alone", userPref: &ru, want: "ru"},
		{name: "unsupported preference falls back to the header", userPref: &unknown, acceptLanguage: "lv", want: "lv"},
		{name: "header is used when no preference is stored", acceptLanguage: "ru-RU,ru;q=0.9,en;q=0.8", want: "ru"},
		{name: "regional variant matches the base language", acceptLanguage: "lv-LV", want: "lv"},
		{name: "unsupported header falls back to english", acceptLanguage: "de-DE,de;q=0.9", want: "en"},
		{name: "malformed header falls back to english", acceptLanguage: ";;;", want: "en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := b.Match(tt.userPref, tt.acceptLanguage); got != tt.want {
				t.Fatalf("Match = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrinter(t *testing.T) {
	b, err := i18n.New()
	if err != nil {
		t.Fatalf("loading translations: %v", err)
	}

	en := b.Printer("en")
	lv := b.Printer("lv")
	ru := b.Printer("ru")

	if en.T("login.submit") != "Sign in" {
		t.Fatalf("en login.submit = %q", en.T("login.submit"))
	}
	// Every locale must actually differ from English for a visible string, otherwise a
	// copy-pasted catalogue would pass unnoticed.
	for _, p := range []*i18n.Printer{lv, ru} {
		if p.T("login.submit") == en.T("login.submit") {
			t.Fatalf("%s login.submit is identical to english", p.Locale())
		}
		if strings.TrimSpace(p.T("error.forbidden")) == "" {
			t.Fatalf("%s error.forbidden is empty", p.Locale())
		}
	}

	// An unknown locale degrades to the fallback, an unknown key to the key itself:
	// a missing translation never breaks a page.
	if got := b.Printer("de").T("login.submit"); got != "Sign in" {
		t.Fatalf("unknown locale = %q, want the english text", got)
	}
	if got := en.T("nope.not.a.key"); got != "nope.not.a.key" {
		t.Fatalf("unknown key = %q, want the key back", got)
	}
}
