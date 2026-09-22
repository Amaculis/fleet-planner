package repo

import (
	"context"
	"fmt"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

// Both queries select every column of users in table order, so sqlc returns the
// generated model type sqlcgen.User for each.

// GetUserByEmail returns domain.ErrNotFound when no account exists. The caller must not
// let that difference become observable: the login handler runs a dummy password
// verification in that case.
func (r *Repo) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return domain.User{}, fmt.Errorf("getting user by email: %w", translate(err))
	}
	return userFromRow(row)
}

func (r *Repo) GetUserByID(ctx context.Context, id int64) (domain.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return domain.User{}, fmt.Errorf("getting user by id: %w", translate(err))
	}
	return userFromRow(row)
}

// CreateUser inserts an account. The caller passes an already-hashed password; the DB
// CHECK rejects anything that is not an argon2id PHC string.
func (r *Repo) CreateUser(ctx context.Context, email, passwordHash string, role domain.Role, driverID *int64, locale *string) (domain.User, error) {
	row, err := r.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
		Role:         sqlcgen.UserRole(role),
		DriverID:     driverID,
		Locale:       locale,
	})
	if err != nil {
		return domain.User{}, fmt.Errorf("creating user: %w", translate(err))
	}
	return userFromRow(row)
}

func (r *Repo) ListUsers(ctx context.Context) ([]domain.UserListItem, error) {
	rows, err := r.q.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", translate(err))
	}
	users := make([]domain.UserListItem, 0, len(rows))
	for _, row := range rows {
		user, err := userFromRow(sqlcgen.User{
			ID:           row.ID,
			Email:        row.Email,
			PasswordHash: row.PasswordHash,
			Role:         row.Role,
			DriverID:     row.DriverID,
			Locale:       row.Locale,
			IsActive:     row.IsActive,
			CreatedAt:    row.CreatedAt,
			UpdatedAt:    row.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		users = append(users, domain.UserListItem{User: user, DriverName: row.DriverName})
	}
	return users, nil
}

// SetUserActive deactivates or reactivates an account. The service deletes the user's
// sessions in the same transaction when deactivating, so access stops immediately.
func (r *Repo) SetUserActive(ctx context.Context, id int64, active bool) (domain.User, error) {
	row, err := r.q.SetUserActive(ctx, sqlcgen.SetUserActiveParams{ID: id, IsActive: active})
	if err != nil {
		return domain.User{}, fmt.Errorf("setting user active: %w", translate(err))
	}
	return userFromRow(row)
}

// SetUserPassword stores an already-hashed password (argon2id, DB CHECK enforced).
func (r *Repo) SetUserPassword(ctx context.Context, id int64, passwordHash string) (domain.User, error) {
	row, err := r.q.SetUserPassword(ctx, sqlcgen.SetUserPasswordParams{ID: id, PasswordHash: passwordHash})
	if err != nil {
		return domain.User{}, fmt.Errorf("setting user password: %w", translate(err))
	}
	return userFromRow(row)
}

// CountUsers supports the create-admin bootstrap ("is this a fresh install?").
func (r *Repo) CountUsers(ctx context.Context) (int64, error) {
	n, err := r.q.CountUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting users: %w", translate(err))
	}
	return n, nil
}

func userFromRow(row sqlcgen.User) (domain.User, error) {
	role, ok := domain.ParseRole(string(row.Role))
	if !ok {
		// Only reachable if the enum gained a value this build does not know about.
		return domain.User{}, fmt.Errorf("unknown role for user %d", row.ID)
	}
	return domain.User{
		ID:           row.ID,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Role:         role,
		DriverID:     row.DriverID,
		Locale:       row.Locale,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}
