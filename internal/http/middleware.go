package http

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// maxRequestBody caps any request body the app will read. Caddy enforces a limit too;
// this one protects a direct-to-app deployment and development.
const maxRequestBody = 1 << 20 // 1 MiB

// Recover turns a panic into a 500 without leaking the panic value or stack to the
// client. The stack goes to the log, tied to the request id.
func (s *Server) Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic recovered",
					"request_id", RequestIDFrom(r.Context()),
					"method", r.Method,
					"path", r.URL.Path,
					"panic", fmt.Sprint(rec),
				)
				s.renderError(w, r, http.StatusInternalServerError, "error.server")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestID assigns an id used in logs and audit entries. A client-supplied value is
// ignored: it would let a caller poison log correlation.
func (s *Server) RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, 12)
		if _, err := rand.Read(raw); err != nil {
			s.log.Error("generating request id", "error", err)
		}
		id := base64.RawURLEncoding.EncodeToString(raw)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, withValue(r, ctxKeyRequestID, id))
	})
}

// SecurityHeaders sets the response headers that are the app's responsibility. HSTS is
// set by Caddy, which terminates TLS.
//
// The CSP is nonce-based and allow-lists nothing external: no CDN, no inline handler.
// htmx is served from /static, so 'self' covers it.
func (s *Server) SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			s.log.Error("generating csp nonce", "error", err)
			s.renderError(w, r, http.StatusInternalServerError, "error.server")
			return
		}
		nonce := base64.RawStdEncoding.EncodeToString(raw)

		h := w.Header()
		h.Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'none'",
			"script-src 'self' 'nonce-" + nonce + "'",
			// The nonce covers the timeline's generated position rules; there is still
			// no unsafe-inline, so an injected style attribute cannot apply.
			"style-src 'self' 'nonce-" + nonce + "'",
			"img-src 'self' data:",
			"font-src 'self'",
			"connect-src 'self'",
			"form-action 'self'",
			"frame-ancestors 'none'",
			"base-uri 'none'",
			"manifest-src 'self'",
		}, "; "))
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=(), interest-cohort=()")
		// Authenticated pages must never be cached by a shared proxy.
		h.Set("Cache-Control", "no-store")

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		next.ServeHTTP(w, withValue(r, ctxKeyCSPNonce, nonce))
	})
}

// clientIP resolves the caller's address. X-Forwarded-For is trusted only when
// TRUST_PROXY_HEADERS is on, which is only correct behind a proxy that overwrites it
// (our Caddy does). Otherwise any client could spoof its way past the rate limiter.
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxyHeaders {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if ip := net.ParseIP(first); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}
