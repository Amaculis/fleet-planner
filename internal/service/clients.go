package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo"
)

// ClientService owns clients: who a trip is for. Planners (admin, dispatcher) manage
// them; erasure is admin-only. Drivers have no access at all.
type ClientService struct {
	repo *repo.Repo
	log  *slog.Logger
}

func NewClientService(r *repo.Repo, log *slog.Logger) *ClientService {
	return &ClientService{repo: r, log: log}
}

func (s *ClientService) List(ctx context.Context, actor domain.Identity) ([]domain.Client, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	return s.repo.ListClients(ctx)
}

func (s *ClientService) ListSelectable(ctx context.Context, actor domain.Identity) ([]domain.Client, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	return s.repo.ListActiveClients(ctx)
}

func (s *ClientService) Get(ctx context.Context, actor domain.Identity, id int64) (domain.Client, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Client{}, err
	}
	return s.repo.GetClient(ctx, id)
}

func (s *ClientService) Create(ctx context.Context, actor domain.Identity, input domain.Client, meta Meta) (domain.Client, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Client{}, err
	}
	input, err := validateClient(input)
	if err != nil {
		return domain.Client{}, err
	}

	var created domain.Client
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		client, err := tx.CreateClient(ctx, input)
		if err != nil {
			return err
		}
		created = client
		return NewAuditor(tx).Record(ctx, &actor.UserID, "create", "client", &client.ID, nil, snapshotClient(client), meta)
	})
	if err != nil {
		return domain.Client{}, fmt.Errorf("creating client: %w", err)
	}
	return created, nil
}

func (s *ClientService) Update(ctx context.Context, actor domain.Identity, input domain.Client, meta Meta) (domain.Client, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Client{}, err
	}
	input, err := validateClient(input)
	if err != nil {
		return domain.Client{}, err
	}

	var updated domain.Client
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetClient(ctx, input.ID)
		if err != nil {
			return err
		}
		if before.IsAnonymized() {
			return Message(domain.ErrConflict, "error.client_erased")
		}
		after, err := tx.UpdateClient(ctx, input)
		if err != nil {
			return err
		}
		updated = after
		return NewAuditor(tx).Record(ctx, &actor.UserID, "update", "client", &after.ID,
			snapshotClient(before), snapshotClient(after), meta)
	})
	if err != nil {
		return domain.Client{}, fmt.Errorf("updating client: %w", err)
	}
	return updated, nil
}

// Anonymize is the GDPR right-to-erasure path for a client: identifying columns are
// cleared and the row kept, so the trips that reference it stay intact. Irreversible.
func (s *ClientService) Anonymize(ctx context.Context, actor domain.Identity, id int64, meta Meta) (domain.Client, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.Client{}, err
	}

	var erased domain.Client
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetClient(ctx, id)
		if err != nil {
			return err
		}
		if before.IsAnonymized() {
			return Message(domain.ErrConflict, "error.client_already_erased")
		}
		after, err := tx.AnonymizeClient(ctx, id, fmt.Sprintf("Erased client #%d", id))
		if err != nil {
			return err
		}
		erased = after
		return NewAuditor(tx).Record(ctx, &actor.UserID, "anonymize", "client", &id,
			snapshotClient(before), snapshotClient(after), meta)
	})
	if err != nil {
		return domain.Client{}, fmt.Errorf("anonymizing client: %w", err)
	}
	return erased, nil
}

// checkTripClient confirms a trip may point at clientID. A client that is merely being
// kept (unchanged from previous) is always fine; a newly chosen one must exist and be
// active, so a deactivated or erased client cannot be attached to fresh work.
func checkTripClient(ctx context.Context, tx *repo.Repo, clientID, previous *int64) error {
	if clientID == nil {
		return nil
	}
	if previous != nil && *previous == *clientID {
		return nil
	}
	client, err := tx.GetClient(ctx, *clientID)
	if errors.Is(err, domain.ErrNotFound) {
		return unknownValue("field.client")
	}
	if err != nil {
		return err
	}
	if !client.IsActive {
		return Message(domain.ErrValidation, "error.client_inactive")
	}
	return nil
}
