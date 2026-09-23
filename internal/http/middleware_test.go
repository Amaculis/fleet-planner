package http

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/config"
	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/i18n"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	bundle, err := i18n.New()
	if err != nil {
		t.Fatalf("loading translations: %v", err)
	}
	cfg := config.Config{
		Env:       "development",
		BaseURL:   "http://localhost:8080",
		AppSecret: []byte("0123456789abcdef0123456789abcdef"),
	}
	return NewServer(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), Services{}, bundle, nil)
}

// requestAs builds a request that already carries an identity and a printer, as
// LoadSession would have left it.
func requestAs(t *testing.T, s *Server, method, path string, identity *domain.Identity) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(r.Context(), ctxKeyPrinter, s.i18n.Printer(i18n.Fallback))
	if identity != nil {
		ctx = context.WithValue(ctx, ctxKeyIdentity, *identity)
		ctx = context.WithValue(ctx, ctxKeyCSRFToken, auth.SessionCSRFToken(identity.CSRFSecret))
	}
	return r.WithContext(ctx)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("handler ran"))
	})
}

func identityFor(role domain.Role, driverID *int64) *domain.Identity {
	return &domain.Identity{
		UserID:     1,
		Email:      "someone@example.com",
		Role:       role,
		DriverID:   driverID,
		CSRFSecret: []byte("secret-secret-secret-secret-1234"),
	}
}

func TestRequireRole(t *testing.T) {
	s := testServer(t)
	driverID := int64(9)

	tests := []struct {
		name     string
		allowed  []domain.Role
		identity *domain.Identity
		wantCode int
	}{
		{name: "admin reaches admin-only route", allowed: []domain.Role{domain.RoleAdmin},
			identity: identityFor(domain.RoleAdmin, nil), wantCode: http.StatusOK},
		{name: "dispatcher blocked from admin-only route", allowed: []domain.Role{domain.RoleAdmin},
			identity: identityFor(domain.RoleDispatcher, nil), wantCode: http.StatusForbidden},
		{name: "driver blocked from admin-only route", allowed: []domain.Role{domain.RoleAdmin},
			identity: identityFor(domain.RoleDriver, &driverID), wantCode: http.StatusForbidden},
		{name: "driver blocked from dispatch route", allowed: []domain.Role{domain.RoleAdmin, domain.RoleDispatcher},
			identity: identityFor(domain.RoleDriver, &driverID), wantCode: http.StatusForbidden},
		{name: "dispatcher reaches dispatch route", allowed: []domain.Role{domain.RoleAdmin, domain.RoleDispatcher},
			identity: identityFor(domain.RoleDispatcher, nil), wantCode: http.StatusOK},
		{name: "driver reaches driver route", allowed: []domain.Role{domain.RoleDriver},
			identity: identityFor(domain.RoleDriver, &driverID), wantCode: http.StatusOK},
		{name: "admin blocked from driver-only route", allowed: []domain.Role{domain.RoleDriver},
			identity: identityFor(domain.RoleAdmin, nil), wantCode: http.StatusForbidden},
		{name: "anonymous is sent to login", allowed: []domain.Role{domain.RoleAdmin},
			identity: nil, wantCode: http.StatusSeeOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.RequireRole(tt.allowed...)(okHandler()).ServeHTTP(rec, requestAs(t, s, http.MethodGet, "/guarded", tt.identity))

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if tt.wantCode != http.StatusOK && strings.Contains(rec.Body.String(), "handler ran") {
				t.Fatal("the guarded handler ran despite the role check failing")
			}
		})
	}
}

func TestRequireAuth(t *testing.T) {
	s := testServer(t)

	t.Run("anonymous browser request redirects to login", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.RequireAuth(okHandler()).ServeHTTP(rec, requestAs(t, s, http.MethodGet, "/", nil))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("status = %d, want 303", rec.Code)
		}
		if got := rec.Header().Get("Location"); got != "/login" {
			t.Fatalf("Location = %q, want /login", got)
		}
	})

	t.Run("anonymous htmx request gets HX-Redirect", func(t *testing.T) {
		rec := httptest.NewRecorder()
		r := requestAs(t, s, http.MethodGet, "/", nil)
		r.Header.Set("HX-Request", "true")
		s.RequireAuth(okHandler()).ServeHTTP(rec, r)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if got := rec.Header().Get("HX-Redirect"); got != "/login" {
			t.Fatalf("HX-Redirect = %q, want /login", got)
		}
	})

	t.Run("authenticated request passes through", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.RequireAuth(okHandler()).ServeHTTP(rec, requestAs(t, s, http.MethodGet, "/", identityFor(domain.RoleAdmin, nil)))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})
}

func TestCSRF(t *testing.T) {
	s := testServer(t)
	identity := identityFor(domain.RoleDispatcher, nil)
	validToken := auth.SessionCSRFToken(identity.CSRFSecret)

	post := func(t *testing.T, form url.Values, header string) *httptest.ResponseRecorder {
		t.Helper()
		body := strings.NewReader(form.Encode())
		r := httptest.NewRequest(http.MethodPost, "/trips", body)
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if header != "" {
			r.Header.Set("X-CSRF-Token", header)
		}
		ctx := context.WithValue(r.Context(), ctxKeyPrinter, s.i18n.Printer(i18n.Fallback))
		ctx = context.WithValue(ctx, ctxKeyIdentity, *identity)
		ctx = context.WithValue(ctx, ctxKeyCSRFToken, validToken)

		rec := httptest.NewRecorder()
		s.CSRF(okHandler()).ServeHTTP(rec, r.WithContext(ctx))
		return rec
	}

	t.Run("post without a token is refused", func(t *testing.T) {
		rec := post(t, url.Values{"origin": {"Riga"}}, "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "handler ran") {
			t.Fatal("handler ran without a CSRF token")
		}
	})

	t.Run("post with a forged token is refused", func(t *testing.T) {
		rec := post(t, url.Values{"csrf_token": {auth.SessionCSRFToken([]byte("some-other-session-secret-32-b!!"))}}, "")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
	})

	t.Run("post with the form token passes", func(t *testing.T) {
		rec := post(t, url.Values{"csrf_token": {validToken}}, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("htmx header token passes", func(t *testing.T) {
		rec := post(t, url.Values{}, validToken)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("get is exempt", func(t *testing.T) {
		rec := httptest.NewRecorder()
		s.CSRF(okHandler()).ServeHTTP(rec, requestAs(t, s, http.MethodGet, "/trips", identity))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})
}

func TestSecurityHeaders(t *testing.T) {
	s := testServer(t)
	rec := httptest.NewRecorder()
	s.SecurityHeaders(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
		"Cache-Control":          "no-store",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'none'", "frame-ancestors 'none'", "base-uri 'none'", "form-action 'self'", "nonce-"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP %q is missing %q", csp, directive)
		}
	}
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP allows unsafe script execution: %q", csp)
	}
}

// TestSecurityHeadersRelaxesStyleSrcOnlyForThePortal: lx-ui applies styling via
// runtime :style bindings that no nonce can cover, so /app/ trades its style-src
// nonce for 'unsafe-inline' there (see the comment in SecurityHeaders). Every other
// route — including the script-src nonce, everywhere — must stay exactly as strict
// as before.
func TestSecurityHeadersRelaxesStyleSrcOnlyForThePortal(t *testing.T) {
	s := testServer(t)

	rec := httptest.NewRecorder()
	s.SecurityHeaders(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/app/buses", nil))
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "style-src 'self' 'unsafe-inline'") {
		t.Errorf("CSP for /app/ = %q, want a style-src with 'unsafe-inline' and no nonce", csp)
	}
	if !strings.Contains(csp, "script-src 'self' 'nonce-") {
		t.Errorf("CSP for /app/ = %q, script-src must still be nonce-only", csp)
	}

	rec2 := httptest.NewRecorder()
	s.SecurityHeaders(okHandler()).ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/trips", nil))
	csp2 := rec2.Header().Get("Content-Security-Policy")
	if strings.Contains(csp2, "unsafe-inline") {
		t.Errorf("CSP for /trips = %q, the server-rendered app must not get the relaxed style-src", csp2)
	}
	if !strings.Contains(csp2, "style-src 'self' 'nonce-") {
		t.Errorf("CSP for /trips = %q, want a nonce-only style-src", csp2)
	}
}

func TestClientIPTrustsProxyOnlyWhenConfigured(t *testing.T) {
	s := testServer(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.5:34567"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.1")

	// Untrusted: a client cannot pick its own rate-limit bucket.
	if got := s.clientIP(r); got != "10.0.0.5" {
		t.Fatalf("clientIP = %q, want the peer address 10.0.0.5", got)
	}

	s.cfg.TrustProxyHeaders = true
	if got := s.clientIP(r); got != "203.0.113.9" {
		t.Fatalf("clientIP = %q, want 203.0.113.9 from X-Forwarded-For", got)
	}
}

func TestRateLimiter(t *testing.T) {
	l := NewLimiter(time.Hour, 3) // 3 up front, then one per hour

	for i := 0; i < 3; i++ {
		if !l.Allow("198.51.100.7") {
			t.Fatalf("request %d was limited but should have been allowed", i+1)
		}
	}
	if l.Allow("198.51.100.7") {
		t.Fatal("the 4th request was allowed; the burst is not enforced")
	}
	// Other clients are unaffected.
	if !l.Allow("198.51.100.8") {
		t.Fatal("a different IP was limited")
	}
	// An unresolvable address fails closed.
	if l.Allow("") {
		t.Fatal("an empty key was allowed")
	}
}
