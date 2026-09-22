// Package http contains handlers and middleware only: HTTP parsing, authorization
// wiring and rendering. Business logic lives in internal/service.
package http

import (
	"context"
	"net/http"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/i18n"
)

type ctxKey int

const (
	ctxKeyIdentity ctxKey = iota
	ctxKeyRequestID
	ctxKeyCSPNonce
	ctxKeyPrinter
	ctxKeyCSRFToken
	ctxKeySessionToken
)

// IdentityFrom returns the authenticated caller. The second result is false for
// anonymous requests. This is the only source of truth for authorization — never a
// header, query parameter or form field.
func IdentityFrom(ctx context.Context) (domain.Identity, bool) {
	id, ok := ctx.Value(ctxKeyIdentity).(domain.Identity)
	return id, ok
}

// MustIdentity is for handlers mounted behind RequireAuth, where anonymous access is
// already impossible.
func MustIdentity(ctx context.Context) domain.Identity {
	id, ok := IdentityFrom(ctx)
	if !ok {
		panic("http: MustIdentity called on an unauthenticated request; check middleware order")
	}
	return id
}

func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

func cspNonceFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyCSPNonce).(string)
	return v
}

// PrinterFrom returns the translator for this request's locale.
func PrinterFrom(ctx context.Context) *i18n.Printer {
	v, _ := ctx.Value(ctxKeyPrinter).(*i18n.Printer)
	return v
}

// CSRFTokenFrom returns the token every form in the response must embed.
func CSRFTokenFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyCSRFToken).(string)
	return v
}

func sessionTokenFrom(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeySessionToken).(string)
	return v
}

func withValue(r *http.Request, key ctxKey, val any) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), key, val))
}
