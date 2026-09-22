package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// SessionTokenBytes is the entropy of the opaque cookie token.
const SessionTokenBytes = 32

// SessionStore is the persistence the manager needs. Implemented by internal/repo.
type SessionStore interface {
	CreateSession(ctx context.Context, s domain.Session) error
	// GetAndTouchSession resolves a live session and refreshes its idle clock.
	// It must apply absolute expiry, idle expiry (idleCutoff) and the user's is_active
	// flag in SQL, and return domain.ErrNotFound when no row qualifies.
	GetAndTouchSession(ctx context.Context, tokenHash []byte, idleCutoff time.Time) (domain.Identity, error)
	DeleteSession(ctx context.Context, tokenHash []byte) error
	DeleteSessionsForUser(ctx context.Context, userID int64) error
	DeleteExpiredSessions(ctx context.Context, idleCutoff time.Time) (int64, error)
}

// Manager owns session lifecycle. It never touches net/http: the caller in
// internal/http turns the returned token into a cookie.
type Manager struct {
	store    SessionStore
	idle     time.Duration
	absolute time.Duration
}

func NewManager(store SessionStore, idle, absolute time.Duration) *Manager {
	return &Manager{store: store, idle: idle, absolute: absolute}
}

func (m *Manager) IdleTimeout() time.Duration     { return m.idle }
func (m *Manager) AbsoluteTimeout() time.Duration { return m.absolute }

// Create issues a new session and returns the raw token, which is shown to the client
// exactly once (in the cookie). Only its SHA-256 is persisted, so a database or backup
// leak yields no usable cookie.
//
// Callers must call this *after* a successful login and must not reuse any pre-login
// identifier: a fresh token is what makes session fixation impossible.
func (m *Manager) Create(ctx context.Context, userID int64) (token string, expiresAt time.Time, err error) {
	raw := make([]byte, SessionTokenBytes)
	if _, err = rand.Read(raw); err != nil {
		return "", time.Time{}, fmt.Errorf("generating session token: %w", err)
	}
	csrfSecret := make([]byte, 32)
	if _, err = rand.Read(csrfSecret); err != nil {
		return "", time.Time{}, fmt.Errorf("generating csrf secret: %w", err)
	}

	token = base64.RawURLEncoding.EncodeToString(raw)
	expiresAt = time.Now().UTC().Add(m.absolute)

	if err = m.store.CreateSession(ctx, domain.Session{
		TokenHash:  HashToken(token),
		UserID:     userID,
		CSRFSecret: csrfSecret,
		ExpiresAt:  expiresAt,
	}); err != nil {
		return "", time.Time{}, fmt.Errorf("storing session: %w", err)
	}
	return token, expiresAt, nil
}

// Resolve validates a token and returns the caller's identity. A token that is unknown,
// expired (idle or absolute) or whose user has been deactivated yields
// domain.ErrUnauthorized.
func (m *Manager) Resolve(ctx context.Context, token string) (domain.Identity, error) {
	if !plausibleToken(token) {
		return domain.Identity{}, domain.ErrUnauthorized
	}
	idleCutoff := time.Now().UTC().Add(-m.idle)
	id, err := m.store.GetAndTouchSession(ctx, HashToken(token), idleCutoff)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Identity{}, domain.ErrUnauthorized
		}
		return domain.Identity{}, fmt.Errorf("resolving session: %w", err)
	}
	return id, nil
}

// Destroy invalidates a single session server-side (logout).
func (m *Manager) Destroy(ctx context.Context, token string) error {
	if !plausibleToken(token) {
		return nil
	}
	if err := m.store.DeleteSession(ctx, HashToken(token)); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// DestroyAllForUser logs a user out everywhere (password change, deactivation).
func (m *Manager) DestroyAllForUser(ctx context.Context, userID int64) error {
	if err := m.store.DeleteSessionsForUser(ctx, userID); err != nil {
		return fmt.Errorf("deleting sessions for user: %w", err)
	}
	return nil
}

// PurgeExpired removes rows that can no longer authenticate anything. Expiry is already
// enforced on every read; this only keeps the table small.
func (m *Manager) PurgeExpired(ctx context.Context) (int64, error) {
	n, err := m.store.DeleteExpiredSessions(ctx, time.Now().UTC().Add(-m.idle))
	if err != nil {
		return 0, fmt.Errorf("purging sessions: %w", err)
	}
	return n, nil
}

// HashToken maps a raw token to the value stored in sessions.token_hash.
// SHA-256 is right here (not argon2): the token is 32 random bytes, so there is no
// guessable input to slow an attacker down over.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// plausibleToken rejects obviously malformed cookie values before they reach the DB.
func plausibleToken(token string) bool {
	if len(token) != base64.RawURLEncoding.EncodedLen(SessionTokenBytes) {
		return false
	}
	_, err := base64.RawURLEncoding.Strict().DecodeString(token)
	return err == nil
}
