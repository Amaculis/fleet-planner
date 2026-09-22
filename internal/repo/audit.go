package repo

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

// InsertAuditEntry appends one row to the append-only audit log. Call it inside
// Repo.InTx together with the mutation it describes — the app's DB role has
// INSERT/SELECT on audit_log and no UPDATE or DELETE, so history cannot be rewritten
// even if the application is compromised.
func (r *Repo) InsertAuditEntry(ctx context.Context, e domain.AuditEntry) error {
	params := sqlcgen.InsertAuditEntryParams{
		ActorUserID: e.ActorUserID,
		Action:      e.Action,
		Entity:      e.Entity,
		EntityID:    e.EntityID,
		Before:      e.Before,
		After:       e.After,
	}
	if e.RequestID != "" {
		id := e.RequestID
		params.RequestID = &id
	}
	if e.IP != nil {
		// A malformed address is dropped rather than failing the mutation: the audit row
		// matters more than the IP field.
		if addr, err := netip.ParseAddr(*e.IP); err == nil {
			params.Ip = &addr
		}
	}
	if _, err := r.q.InsertAuditEntry(ctx, params); err != nil {
		return fmt.Errorf("inserting audit entry: %w", translate(err))
	}
	return nil
}
