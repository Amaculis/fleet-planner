package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// CSRF uses synchronizer tokens, not a bare double-submit cookie: the token is an HMAC
// of a secret the client never sees, so an attacker who can set cookies (subdomain,
// MITM on a sibling host) still cannot forge a matching form value.
//
// Two cases:
//   - Authenticated: the key is the session's csrf_secret column. The token dies with
//     the session and rotates on login.
//   - Anonymous (the login form itself): the key is the app secret and the message is a
//     per-visitor nonce held in a short-lived cookie.
const (
	csrfSessionContext = "fleet-csrf-session-v1"
	csrfAnonContext    = "fleet-csrf-anon-v1"
	// CSRFNonceBytes is the entropy of the anonymous CSRF nonce cookie.
	CSRFNonceBytes = 16
)

// SessionCSRFToken derives the token for an authenticated session.
func SessionCSRFToken(csrfSecret []byte) string {
	return token(csrfSecret, []byte(csrfSessionContext))
}

// AnonCSRFToken derives the token for a visitor who has no session yet.
func AnonCSRFToken(appSecret, nonce []byte) string {
	return token(appSecret, append([]byte(csrfAnonContext), nonce...))
}

// NewCSRFNonce returns a fresh nonce for the anonymous CSRF cookie.
func NewCSRFNonce() (string, error) {
	raw := make([]byte, CSRFNonceBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating csrf nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// ValidCSRFToken compares in constant time. Both arguments are compared as raw strings;
// a malformed or empty submitted token simply fails.
func ValidCSRFToken(expected, submitted string) bool {
	if expected == "" || submitted == "" {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(submitted))
}

func token(key, msg []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(msg)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
