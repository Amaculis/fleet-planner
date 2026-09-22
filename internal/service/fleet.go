package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo"
)

// FleetService owns buses and drivers: the master data an admin maintains.
type FleetService struct {
	repo     *repo.Repo
	sessions *auth.Manager
	log      *slog.Logger
}

func NewFleetService(r *repo.Repo, sessions *auth.Manager, log *slog.Logger) *FleetService {
	return &FleetService{repo: r, sessions: sessions, log: log}
}

// requireRole is defence in depth. RequireRole middleware already guards every route;
// this makes a mis-wired route fail closed instead of silently allowing a mutation.
func requireRole(actor domain.Identity, allowed ...domain.Role) error {
	for _, role := range allowed {
		if actor.Role == role {
			return nil
		}
	}
	return domain.ErrForbidden
}

// ---------------------------------------------------------------------------
// Buses
// ---------------------------------------------------------------------------

func (s *FleetService) ListBuses(ctx context.Context, actor domain.Identity) ([]domain.Bus, error) {
	// Dispatchers need to read buses to plan; only admins may change them.
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	return s.repo.ListBuses(ctx)
}

func (s *FleetService) ListAssignableBuses(ctx context.Context, actor domain.Identity) ([]domain.Bus, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	return s.repo.ListActiveBuses(ctx)
}

func (s *FleetService) GetBus(ctx context.Context, actor domain.Identity, id int64) (domain.Bus, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Bus{}, err
	}
	return s.repo.GetBus(ctx, id)
}

func (s *FleetService) CreateBus(ctx context.Context, actor domain.Identity, input domain.Bus, meta Meta) (domain.Bus, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.Bus{}, err
	}
	input, err := validateBus(input)
	if err != nil {
		return domain.Bus{}, err
	}

	var created domain.Bus
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		bus, err := tx.CreateBus(ctx, input)
		if err != nil {
			return err
		}
		created = bus
		return NewAuditor(tx).Record(ctx, &actor.UserID, "create", "bus", &bus.ID, nil, snapshotBus(bus), meta)
	})
	if err != nil {
		return domain.Bus{}, fmt.Errorf("creating bus: %w", err)
	}
	return created, nil
}

func (s *FleetService) UpdateBus(ctx context.Context, actor domain.Identity, input domain.Bus, meta Meta) (domain.Bus, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.Bus{}, err
	}
	input, err := validateBus(input)
	if err != nil {
		return domain.Bus{}, err
	}

	var updated domain.Bus
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetBus(ctx, input.ID)
		if err != nil {
			return err
		}
		after, err := tx.UpdateBus(ctx, input)
		if err != nil {
			return err
		}
		updated = after
		return NewAuditor(tx).Record(ctx, &actor.UserID, "update", "bus", &after.ID,
			snapshotBus(before), snapshotBus(after), meta)
	})
	if err != nil {
		return domain.Bus{}, fmt.Errorf("updating bus: %w", err)
	}
	return updated, nil
}

// DeleteBus only succeeds for a bus that was never assigned; the FK is RESTRICT.
// A bus with history is retired (status = 'retired') instead.
func (s *FleetService) DeleteBus(ctx context.Context, actor domain.Identity, id int64, meta Meta) error {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return err
	}
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetBus(ctx, id)
		if err != nil {
			return err
		}
		if err := tx.DeleteBus(ctx, id); err != nil {
			return err
		}
		return NewAuditor(tx).Record(ctx, &actor.UserID, "delete", "bus", &id, snapshotBus(before), nil, meta)
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			// Foreign key violation: the bus has assignments.
			return Message(domain.ErrConflict, "error.bus_has_trips")
		}
		return fmt.Errorf("deleting bus: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Drivers
// ---------------------------------------------------------------------------

func (s *FleetService) ListDrivers(ctx context.Context, actor domain.Identity) ([]domain.Driver, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	return s.repo.ListDrivers(ctx)
}

func (s *FleetService) ListAssignableDrivers(ctx context.Context, actor domain.Identity) ([]domain.Driver, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	return s.repo.ListActiveDrivers(ctx)
}

// GetDriver lets admins and dispatchers read any driver, and a driver read only their
// own record — resolved from the session, never from the id in the URL.
func (s *FleetService) GetDriver(ctx context.Context, actor domain.Identity, id int64) (domain.Driver, error) {
	if !actor.OwnsDriver(id) {
		return domain.Driver{}, domain.ErrForbidden
	}
	return s.repo.GetDriver(ctx, id)
}

func (s *FleetService) CreateDriver(ctx context.Context, actor domain.Identity, input domain.Driver, meta Meta) (domain.Driver, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.Driver{}, err
	}
	input, err := validateDriver(input)
	if err != nil {
		return domain.Driver{}, err
	}

	var created domain.Driver
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		driver, err := tx.CreateDriver(ctx, input)
		if err != nil {
			return err
		}
		created = driver
		return NewAuditor(tx).Record(ctx, &actor.UserID, "create", "driver", &driver.ID, nil, snapshotDriver(driver), meta)
	})
	if err != nil {
		return domain.Driver{}, fmt.Errorf("creating driver: %w", err)
	}
	return created, nil
}

func (s *FleetService) UpdateDriver(ctx context.Context, actor domain.Identity, input domain.Driver, meta Meta) (domain.Driver, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.Driver{}, err
	}
	input, err := validateDriver(input)
	if err != nil {
		return domain.Driver{}, err
	}

	var updated domain.Driver
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetDriver(ctx, input.ID)
		if err != nil {
			return err
		}
		if before.IsAnonymized() {
			return Message(domain.ErrConflict, "error.driver_erased")
		}
		after, err := tx.UpdateDriver(ctx, input)
		if err != nil {
			return err
		}
		updated = after
		return NewAuditor(tx).Record(ctx, &actor.UserID, "update", "driver", &after.ID,
			snapshotDriver(before), snapshotDriver(after), meta)
	})
	if err != nil {
		return domain.Driver{}, fmt.Errorf("updating driver: %w", err)
	}
	return updated, nil
}

// AnonymizeDriver is the GDPR right-to-erasure path. It clears the identifying columns,
// deactivates the driver, disables and logs out their login account — and keeps the row,
// so assignments, worked time (payroll-seam) and the audit trail stay referentially
// intact. Irreversible by design.
func (s *FleetService) AnonymizeDriver(ctx context.Context, actor domain.Identity, id int64, meta Meta) (domain.Driver, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.Driver{}, err
	}

	var (
		erased      domain.Driver
		loginUserID *int64
	)
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetDriver(ctx, id)
		if err != nil {
			return err
		}
		if before.IsAnonymized() {
			return Message(domain.ErrConflict, "error.driver_already_erased")
		}

		placeholder := fmt.Sprintf("Erased driver #%d", id)
		after, err := tx.AnonymizeDriver(ctx, id, placeholder)
		if err != nil {
			return err
		}
		erased = after

		// Deactivate the linked login, if there is one.
		if userID, err := tx.GetDriverLoginUserID(ctx, id); err == nil {
			user, err := tx.SetUserActive(ctx, userID, false)
			if err != nil {
				return err
			}
			loginUserID = &userID
			if err := NewAuditor(tx).Record(ctx, &actor.UserID, "deactivate", "user", &userID,
				nil, snapshotUser(user), meta); err != nil {
				return err
			}
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}

		return NewAuditor(tx).Record(ctx, &actor.UserID, "anonymize", "driver", &id,
			snapshotDriver(before), snapshotDriver(after), meta)
	})
	if err != nil {
		return domain.Driver{}, fmt.Errorf("anonymizing driver: %w", err)
	}

	// Outside the transaction: revoking sessions is not something to roll back.
	if loginUserID != nil {
		if err := s.sessions.DestroyAllForUser(ctx, *loginUserID); err != nil {
			s.log.Error("revoking sessions after erasure", "user_id", *loginUserID, "error", err)
		}
	}
	return erased, nil
}
