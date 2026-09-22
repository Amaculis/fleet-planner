// Package i18n is the translation layer. No UI string is written literally in a handler
// or template: everything goes through a Printer keyed by message id, with en, lv and ru
// provided from the start.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

// Supported locales, most preferred first. The first entry is the fallback.
var supported = []string{"en", "lv", "ru"}

const Fallback = "en"

type Bundle struct {
	messages map[string]map[string]string // locale -> key -> text
	matcher  language.Matcher
	tags     []language.Tag
}

// New loads the embedded catalogues and fails if a locale file is missing or malformed,
// so a broken translation file cannot ship silently.
func New() (*Bundle, error) {
	b := &Bundle{messages: make(map[string]map[string]string, len(supported))}
	for _, loc := range supported {
		raw, err := localeFS.ReadFile("locales/" + loc + ".json")
		if err != nil {
			return nil, fmt.Errorf("reading locale %q: %w", loc, err)
		}
		var msgs map[string]string
		if err := json.Unmarshal(raw, &msgs); err != nil {
			return nil, fmt.Errorf("parsing locale %q: %w", loc, err)
		}
		b.messages[loc] = msgs

		tag, err := language.Parse(loc)
		if err != nil {
			return nil, fmt.Errorf("parsing language tag %q: %w", loc, err)
		}
		b.tags = append(b.tags, tag)
	}
	b.matcher = language.NewMatcher(b.tags)

	if err := b.checkComplete(); err != nil {
		return nil, err
	}
	return b, nil
}

// checkComplete guarantees every locale defines exactly the same keys as the fallback.
func (b *Bundle) checkComplete() error {
	base := b.messages[Fallback]
	var problems []string
	for _, loc := range supported {
		if loc == Fallback {
			continue
		}
		for key := range base {
			if _, ok := b.messages[loc][key]; !ok {
				problems = append(problems, fmt.Sprintf("%s: missing %q", loc, key))
			}
		}
		for key := range b.messages[loc] {
			if _, ok := base[key]; !ok {
				problems = append(problems, fmt.Sprintf("%s: unknown key %q", loc, key))
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return fmt.Errorf("incomplete translations:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

// Supported returns the locale codes, fallback first.
func (b *Bundle) Supported() []string { return append([]string(nil), supported...) }

// Match picks a locale: explicit user preference first, then Accept-Language, then the
// fallback. userPref is the users.locale column (nil when unset).
func (b *Bundle) Match(userPref *string, acceptLanguage string) string {
	if userPref != nil {
		if _, ok := b.messages[*userPref]; ok {
			return *userPref
		}
	}
	if acceptLanguage != "" {
		if tags, _, err := language.ParseAcceptLanguage(acceptLanguage); err == nil && len(tags) > 0 {
			if _, idx, conf := b.matcher.Match(tags...); conf != language.No && idx < len(supported) {
				return supported[idx]
			}
		}
	}
	return Fallback
}

// Printer renders messages for one locale.
type Printer struct {
	bundle *Bundle
	locale string
}

func (b *Bundle) Printer(locale string) *Printer {
	if _, ok := b.messages[locale]; !ok {
		locale = Fallback
	}
	return &Printer{bundle: b, locale: locale}
}

func (p *Printer) Locale() string { return p.locale }

// T returns the translated message, falling back to English and, if the key is unknown
// everywhere, to the key itself — a missing translation degrades the UI, it never
// breaks a page.
func (p *Printer) T(key string, args ...any) string {
	msg, ok := p.bundle.messages[p.locale][key]
	if !ok {
		if msg, ok = p.bundle.messages[Fallback][key]; !ok {
			return key
		}
	}
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}
