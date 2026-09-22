// Package repo is the only package that talks to Postgres. Every statement comes from
// sqlc-generated code (db/queries/*.sql -> internal/repo/sqlcgen); there is no
// hand-written or concatenated SQL anywhere in Go.
//
// Run `make generate` (sqlc + templ) after any schema or query change.
package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

type Repo struct {
	pool *pgxpool.Pool
	q    *sqlcgen.Queries
}

func New(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool, q: sqlcgen.New(pool)}
}

// InTx runs fn inside a single transaction. Multi-statement invariants — above all
// "mutation plus its audit row" — must go through this, so a mutation can never commit
// without its audit entry.
func (r *Repo) InTx(ctx context.Context, fn func(*Repo) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after a successful commit

	if err := fn(&Repo{pool: r.pool, q: r.q.WithTx(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// Ping is used by the readiness/health check.
func (r *Repo) Ping(ctx context.Context) error {
	if err := r.pool.Ping(ctx); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}
	return nil
}

// translate maps driver-level errors to domain sentinels so services and handlers never
// branch on Postgres specifics. Constraint names are the contract:
//
//	assignments_no_bus_overlap / assignments_no_driver_overlap -> domain.ErrTimeConflict
//
// which is the DB-level backstop behind the service's own conflict pre-check.
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23P01": // exclusion_violation
			return fmt.Errorf("%w (%s)", domain.ErrTimeConflict, pgErr.ConstraintName)
		case "23505": // unique_violation
			return fmt.Errorf("%w (%s)", domain.ErrConflict, pgErr.ConstraintName)
		case "23503", "23514": // foreign_key_violation, check_violation
			return fmt.Errorf("%w (%s)", domain.ErrValidation, pgErr.ConstraintName)
		}
	}
	return err
}
