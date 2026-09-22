package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo"
)

// UserService manages login accounts. Admin-only throughout.
type UserService struct {
	repo       *repo.Repo
	sessions   *auth.Manager
	log        *slog.Logger
	hashParams auth.Params
}

func NewUserService(r *repo.Repo, sessions *auth.Manager, log *slog.Logger, hashParams auth.Params) *UserService {
	return &UserService{repo: r, sessions: sessions, log: log, hashParams: hashParams}
}

func (s *UserService) List(ctx context.Context, actor domain.Identity) ([]domain.UserListItem, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return nil, err
	}
	return s.repo.ListUsers(ctx)
}

func (s *UserService) Get(ctx context.Context, actor domain.Identity, id int64) (domain.User, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.User{}, err
	}
	return s.repo.GetUserByID(ctx, id)
}

// Create adds an account. A driver-role account must name the driver record it belongs
// to; the DB CHECK enforces the same rule, so the two cannot drift apart.
func (s *UserService) Create(ctx context.Context, actor domain.Identity, email, password string, role domain.Role, driverID *int64, meta Meta) (domain.User, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.User{}, err
	}

	email = NormalizeEmail(email)
	if err := ValidateEmail(email); err != nil {
		return domain.User{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return domain.User{}, err
	}
	if _, ok := domain.ParseRole(string(role)); !ok {
		return domain.User{}, unknownValue("field.role")
	}
	if (role == domain.RoleDriver) != (driverID != nil) {
		return domain.User{}, Message(domain.ErrValidation, "error.driver_link_mismatch")
	}

	hash, err := auth.HashPassword(password, s.hashParams)
	if err != nil {
		return domain.User{}, fmt.Errorf("hashing password: %w", err)
	}

	var created domain.User
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		if driverID != nil {
			driver, err := tx.GetDriver(ctx, *driverID)
			if err != nil {
				return err
			}
			if !driver.IsActive || driver.IsAnonymized() {
				return Message(domain.ErrValidation, "error.driver_not_active")
			}
		}
		user, err := tx.CreateUser(ctx, email, hash, role, driverID, nil)
		if err != nil {
			return err
		}
		created = user
		return NewAuditor(tx).Record(ctx, &actor.UserID, "create", "user", &user.ID, nil, snapshotUser(user), meta)
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("creating user: %w", err)
	}
	return created, nil
}

// SetActive deactivates or reactivates an account. Deactivating revokes every live
// session immediately — the session lookup itself filters on users.is_active, so access
// stops on the next request even before the rows are deleted.
func (s *UserService) SetActive(ctx context.Context, actor domain.Identity, id int64, active bool, meta Meta) (domain.User, error) {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return domain.User{}, err
	}
	if id == actor.UserID && !active {
		return domain.User{}, Message(domain.ErrValidation, "error.self_deactivate")
	}

	var updated domain.User
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetUserByID(ctx, id)
		if err != nil {
			return err
		}
		after, err := tx.SetUserActive(ctx, id, active)
		if err != nil {
			return err
		}
		updated = after
		action := "activate"
		if !active {
			action = "deactivate"
		}
		return NewAuditor(tx).Record(ctx, &actor.UserID, action, "user", &id,
			snapshotUser(before), snapshotUser(after), meta)
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("setting user active: %w", err)
	}

	if !active {
		if err := s.sessions.DestroyAllForUser(ctx, id); err != nil {
			s.log.Error("revoking sessions after deactivation", "user_id", id, "error", err)
		}
	}
	return updated, nil
}

// SetPassword sets a new password and logs the account out everywhere, so a password
// reset also ends any session an attacker may hold.
func (s *UserService) SetPassword(ctx context.Context, actor domain.Identity, id int64, password string, meta Meta) error {
	if err := requireRole(actor, domain.RoleAdmin); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := auth.HashPassword(password, s.hashParams)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		if _, err := tx.SetUserPassword(ctx, id, hash); err != nil {
			return err
		}
		// The snapshots carry no credential material, only that a change happened.
		return NewAuditor(tx).Record(ctx, &actor.UserID, "password", "user", &id, nil,
			map[string]any{"password_changed": true}, meta)
	})
	if err != nil {
		return fmt.Errorf("setting password: %w", err)
	}

	if err := s.sessions.DestroyAllForUser(ctx, id); err != nil {
		s.log.Error("revoking sessions after password change", "user_id", id, "error", err)
	}
	return nil
}
