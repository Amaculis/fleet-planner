// Command server is the bus fleet application: configuration, dependency wiring,
// graceful shutdown. No business logic lives here.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	// The distroless runtime image has no tzdata; embedding it keeps APP_TIMEZONE working.
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/config"
	apphttp "github.com/buscompany/bus_fleet/internal/http"
	"github.com/buscompany/bus_fleet/internal/i18n"
	"github.com/buscompany/bus_fleet/internal/repo"
	"github.com/buscompany/bus_fleet/internal/service"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck": // used by the container healthcheck: distroless has no curl
			os.Exit(healthcheck())
		case "create-admin":
			if err := createAdmin(); err != nil {
				fmt.Fprintln(os.Stderr, "fatal:", err)
				os.Exit(1)
			}
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q (expected: healthcheck, create-admin)\n", os.Args[1])
			os.Exit(2)
		}
	}
	if err := run(); err != nil {
		// Configuration errors name variables, never values.
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := newLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := newPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	bundle, err := i18n.New()
	if err != nil {
		return fmt.Errorf("loading translations: %w", err)
	}

	repository := repo.New(pool)

	hashParams := auth.DefaultParams()
	dummyHash, err := auth.DummyHash(hashParams)
	if err != nil {
		return fmt.Errorf("preparing password hashing: %w", err)
	}

	sessions := auth.NewManager(repository, cfg.SessionIdleTimeout, cfg.SessionAbsoluteTimeout)
	auditor := service.NewAuditor(repository)

	services := apphttp.Services{
		Auth:        service.NewAuthService(repository, sessions, auditor, log, hashParams, dummyHash),
		Fleet:       service.NewFleetService(repository, sessions, log),
		Trips:       service.NewTripService(repository, log),
		Assignments: service.NewAssignmentService(repository, log),
		Users:       service.NewUserService(repository, sessions, log, hashParams),
	}

	srv := apphttp.NewServer(cfg, log, services, bundle, repository)
	srv.StartBackground(ctx)
	go purgeSessions(ctx, sessions, log)

	httpServer := &stdhttp.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv.Routes(),
		// Slowloris and stuck-client protection.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", cfg.ListenAddr, "env", cfg.Env)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}

func newPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		// Never include the URL itself: it carries the password.
		return nil, errors.New("DATABASE_URL is not a valid Postgres connection string")
	}
	poolCfg.MaxConns = 10
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 15 * time.Minute

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}

// purgeSessions keeps the sessions table from growing without bound. Expiry itself is
// enforced on every read, in SQL, so this is housekeeping rather than security.
func purgeSessions(ctx context.Context, sessions *auth.Manager, log *slog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			purgeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			n, err := sessions.PurgeExpired(purgeCtx)
			cancel()
			if err != nil {
				log.Error("purging expired sessions", "error", err)
				continue
			}
			if n > 0 {
				log.Info("purged expired sessions", "count", n)
			}
		}
	}
}

// newLogger emits structured logs. Handlers must log ids, never personal data: no
// email, name, phone or licence number belongs in application logs — that is what the
// audit log in Postgres is for.
func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// healthcheck probes the local server from inside the container.
func healthcheck() int {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		port = "8080"
	}
	client := &stdhttp.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != stdhttp.StatusOK {
		return 1
	}
	return 0
}
