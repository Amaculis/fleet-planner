package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/config"
	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo"
	"github.com/buscompany/bus_fleet/internal/service"
)

// createAdmin bootstraps the first administrator:
//
//	docker compose run --rm -e ADMIN_EMAIL=... -e ADMIN_PASSWORD=... app create-admin
//
// The password comes from the environment, never from a command-line argument: argv is
// visible to every process on the host. The password is never logged or echoed.
func createAdmin() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg)

	email := service.NormalizeEmail(os.Getenv("ADMIN_EMAIL"))
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || password == "" {
		return errors.New("ADMIN_EMAIL and ADMIN_PASSWORD must both be set")
	}
	if err := service.ValidateEmail(email); err != nil {
		return err
	}
	if err := service.ValidatePassword(password); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := newPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	repository := repo.New(pool)

	hash, err := auth.HashPassword(password, auth.DefaultParams())
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	var created domain.User
	err = repository.InTx(ctx, func(tx *repo.Repo) error {
		// The account and its audit entry commit together, or not at all.
		user, err := tx.CreateUser(ctx, email, hash, domain.RoleAdmin, nil, nil)
		if err != nil {
			return err
		}
		created = user
		// actor is nil: this is a system action with no signed-in user behind it.
		return service.NewAuditor(tx).Record(ctx, nil, "create", "user", &user.ID, nil,
			map[string]any{"email": user.Email, "role": string(user.Role), "source": "create-admin"},
			service.Meta{RequestID: "cli:create-admin"})
	})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return fmt.Errorf("an account with that email already exists")
		}
		return fmt.Errorf("creating admin: %w", err)
	}

	log.Info("admin created", "user_id", created.ID)
	fmt.Printf("created admin user id=%d\n", created.ID)
	return nil
}
