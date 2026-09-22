// Package service holds business logic. It never imports net/http: everything it needs
// about a request arrives as plain values (see Meta).
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// Meta carries the request facts an audit entry needs. The HTTP layer fills it in;
// nothing here knows about headers or cookies.
type Meta struct {
	IP        string // already resolved to a single address by the HTTP layer
	RequestID string
}

// AuditStore is the persistence the auditor needs.
type AuditStore interface {
	InsertAuditEntry(ctx context.Context, e domain.AuditEntry) error
}

// Auditor writes the audit trail. Every mutation goes through it, inside the same
// transaction as the change itself.
type Auditor struct {
	store AuditStore
}

func NewAuditor(store AuditStore) *Auditor { return &Auditor{store: store} }

// Record appends one entry. before/after are marshalled to JSON; pass nil for the side
// that does not exist (creation has no before, deletion has no after).
//
// Callers pass audit-safe views of the entity — never a struct carrying password_hash,
// a session token or a CSRF secret. domain.User must therefore never be passed here
// directly; use an explicit snapshot type.
func (a *Auditor) Record(ctx context.Context, actorUserID *int64, action, entity string, entityID *int64, before, after any, meta Meta) error {
	beforeJSON, err := marshalSnapshot(before)
	if err != nil {
		return fmt.Errorf("marshalling audit 'before': %w", err)
	}
	afterJSON, err := marshalSnapshot(after)
	if err != nil {
		return fmt.Errorf("marshalling audit 'after': %w", err)
	}

	entry := domain.AuditEntry{
		ActorUserID: actorUserID,
		Action:      action,
		Entity:      entity,
		EntityID:    entityID,
		Before:      beforeJSON,
		After:       afterJSON,
		RequestID:   meta.RequestID,
	}
	if meta.IP != "" {
		ip := meta.IP
		entry.IP = &ip
	}
	if err := a.store.InsertAuditEntry(ctx, entry); err != nil {
		return fmt.Errorf("recording audit entry: %w", err)
	}
	return nil
}

func marshalSnapshot(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}
