// Package domain holds the types and sentinel errors shared by every layer
// (http -> service -> repo). It imports nothing from the other internal packages, so it
// cannot create a cycle and it keeps net/http out of the service layer.
package domain

import (
	"errors"
	"time"
)

// Role mirrors the Postgres enum user_role. Roles are compared as values, never parsed
// from client input except through ParseRole.
type Role string

const (
	RoleAdmin      Role = "admin"
	RoleDispatcher Role = "dispatcher"
	RoleDriver     Role = "driver"
)

func ParseRole(s string) (Role, bool) {
	switch Role(s) {
	case RoleAdmin, RoleDispatcher, RoleDriver:
		return Role(s), true
	}
	return "", false
}

// User is a login account. PasswordHash never leaves the repo/auth boundary and is
// never logged or rendered.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         Role
	DriverID     *int64 // set if and only if Role == RoleDriver (DB-enforced)
	Locale       *string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Identity is the authenticated caller, resolved server-side from the session cookie.
// Authorization decisions read this struct and nothing else from the request: a driver's
// DriverID comes from here, never from a URL parameter or form field.
type Identity struct {
	UserID     int64
	Email      string
	Role       Role
	DriverID   *int64
	Locale     *string
	CSRFSecret []byte
	ExpiresAt  time.Time
}

func (i Identity) IsAdmin() bool      { return i.Role == RoleAdmin }
func (i Identity) IsDispatcher() bool { return i.Role == RoleDispatcher }
func (i Identity) IsDriver() bool     { return i.Role == RoleDriver }

// OwnsDriver reports whether this identity may act on the given driver's data.
// Admins and dispatchers may; a driver only for their own record.
func (i Identity) OwnsDriver(driverID int64) bool {
	if i.Role == RoleAdmin || i.Role == RoleDispatcher {
		return true
	}
	return i.DriverID != nil && *i.DriverID == driverID
}

// Session is the server-side record behind the opaque cookie token.
type Session struct {
	TokenHash  []byte
	UserID     int64
	CSRFSecret []byte
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
}

// AuditEntry is one immutable record of a mutation. Before/After are JSON snapshots with
// secrets stripped (never password_hash, never session material).
type AuditEntry struct {
	ActorUserID *int64
	Action      string
	Entity      string
	EntityID    *int64
	Before      []byte
	After       []byte
	IP          *string
	RequestID   string
}

// Sentinel errors. Handlers map these to status codes; the underlying error text is
// logged, never returned to the client.
var (
	ErrNotFound     = errors.New("not found")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict     = errors.New("conflict")
	ErrValidation   = errors.New("validation failed")
	// ErrTimeConflict is the no-double-booking violation (service pre-check or the
	// assignments_no_bus_overlap / assignments_no_driver_overlap EXCLUDE constraint).
	ErrTimeConflict = errors.New("time conflict")
)
