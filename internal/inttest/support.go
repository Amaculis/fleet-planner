// Package inttest holds integration tests that run against a real PostgreSQL instance.
// Mocks cannot prove what this suite is for: the EXCLUDE constraints, the FK cascade
// that keeps an assignment's window in step with its trip, and the ownership filters
// that live inside SQL statements.
//
// The tests skip unless TEST_DATABASE_URL points at a throwaway database with the
// migrations applied (scripts/verify-tests.sh sets one up).
package inttest

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/config"
	"github.com/buscompany/bus_fleet/internal/domain"
	apphttp "github.com/buscompany/bus_fleet/internal/http"
	"github.com/buscompany/bus_fleet/internal/i18n"
	"github.com/buscompany/bus_fleet/internal/repo"
	"github.com/buscompany/bus_fleet/internal/service"
)

// env is one fully wired application, pointed at the test database.
type env struct {
	t           *testing.T
	pool        *pgxpool.Pool
	repo        *repo.Repo
	sessions    *auth.Manager
	fleet       *service.FleetService
	trips       *service.TripService
	assignments *service.AssignmentService
	users       *service.UserService
	server      *apphttp.Server
	cfg         config.Config
}

// hashParams keep the tests fast. Production uses auth.DefaultParams().
func testHashParams() auth.Params {
	p := auth.DefaultParams()
	p.Memory = 8 * 1024
	p.Iterations = 1
	return p
}

func newEnv(t *testing.T) *env {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; run scripts/verify-tests.sh")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connecting to the test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pinging the test database: %v", err)
	}
	t.Cleanup(pool.Close)

	truncate(t, pool)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	bundle, err := i18n.New()
	if err != nil {
		t.Fatalf("loading translations: %v", err)
	}

	cfg := config.Config{
		Env:                    "development",
		BaseURL:                "http://localhost:8080",
		AppSecret:              []byte("0123456789abcdef0123456789abcdef"),
		SessionIdleTimeout:     time.Hour,
		SessionAbsoluteTimeout: 4 * time.Hour,
		Location:               time.UTC,
		// The suite logs in dozens of times from 127.0.0.1; the limiter itself is
		// covered by its own unit test and by scripts/smoke-auth.sh.
		LoginRateBurst:  1000,
		LoginRateEvery:  time.Millisecond,
		GlobalRateBurst: 10000,
		GlobalRateEvery: time.Millisecond,
	}

	r := repo.New(pool)
	sessions := auth.NewManager(r, cfg.SessionIdleTimeout, cfg.SessionAbsoluteTimeout)
	auditor := service.NewAuditor(r)
	dummyHash, err := auth.DummyHash(testHashParams())
	if err != nil {
		t.Fatalf("preparing password hashing: %v", err)
	}

	e := &env{
		t:           t,
		pool:        pool,
		repo:        r,
		sessions:    sessions,
		fleet:       service.NewFleetService(r, sessions, log),
		trips:       service.NewTripService(r, log),
		assignments: service.NewAssignmentService(r, log),
		users:       service.NewUserService(r, sessions, log, testHashParams()),
		cfg:         cfg,
	}
	e.server = apphttp.NewServer(cfg, log, apphttp.Services{
		Auth:        service.NewAuthService(r, sessions, auditor, log, testHashParams(), dummyHash),
		Fleet:       e.fleet,
		Trips:       e.trips,
		Assignments: e.assignments,
		Users:       e.users,
	}, bundle, r)
	return e
}

// truncate empties every table between tests. RESTART IDENTITY keeps ids predictable;
// CASCADE is safe here because the whole graph goes at once.
func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`TRUNCATE assignments, trips, sessions, audit_log, users, drivers, buses RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

func (e *env) newAdmin(email, password string) domain.User {
	e.t.Helper()
	hash, err := auth.HashPassword(password, testHashParams())
	if err != nil {
		e.t.Fatalf("hashing password: %v", err)
	}
	user, err := e.repo.CreateUser(context.Background(), email, hash, domain.RoleAdmin, nil, nil)
	if err != nil {
		e.t.Fatalf("creating admin: %v", err)
	}
	return user
}

func (e *env) newDispatcher(email, password string) domain.User {
	e.t.Helper()
	hash, err := auth.HashPassword(password, testHashParams())
	if err != nil {
		e.t.Fatalf("hashing password: %v", err)
	}
	user, err := e.repo.CreateUser(context.Background(), email, hash, domain.RoleDispatcher, nil, nil)
	if err != nil {
		e.t.Fatalf("creating dispatcher: %v", err)
	}
	return user
}

// newDriverWithLogin creates the driver record and the login account tied to it.
func (e *env) newDriverWithLogin(name, email, password string) (domain.Driver, domain.User) {
	e.t.Helper()
	driver := e.newDriver(name)
	hash, err := auth.HashPassword(password, testHashParams())
	if err != nil {
		e.t.Fatalf("hashing password: %v", err)
	}
	user, err := e.repo.CreateUser(context.Background(), email, hash, domain.RoleDriver, &driver.ID, nil)
	if err != nil {
		e.t.Fatalf("creating driver login: %v", err)
	}
	return driver, user
}

func (e *env) newDriver(name string) domain.Driver {
	e.t.Helper()
	driver, err := e.repo.CreateDriver(context.Background(), domain.Driver{FullName: name, IsActive: true})
	if err != nil {
		e.t.Fatalf("creating driver: %v", err)
	}
	return driver
}

func (e *env) newBus(plate string) domain.Bus {
	e.t.Helper()
	bus, err := e.repo.CreateBus(context.Background(), domain.Bus{
		Plate: plate, Model: "Setra S515", Seats: 50, Status: domain.BusActive,
	})
	if err != nil {
		e.t.Fatalf("creating bus: %v", err)
	}
	return bus
}

// baseDay anchors every trip in the tests to a fixed date, so overlaps are obvious.
var baseDay = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// at returns baseDay at the given hour, in UTC.
func at(hour int) time.Time { return baseDay.Add(time.Duration(hour) * time.Hour) }

func (e *env) newTrip(origin, destination string, startHour, endHour int) domain.Trip {
	e.t.Helper()
	trip, err := e.repo.CreateTrip(context.Background(), domain.Trip{
		Origin:         origin,
		Destination:    destination,
		ScheduledStart: at(startHour),
		ScheduledEnd:   at(endHour),
	})
	if err != nil {
		e.t.Fatalf("creating trip: %v", err)
	}
	return trip
}

func (e *env) identityFor(user domain.User, driverID *int64) domain.Identity {
	return domain.Identity{
		UserID:   user.ID,
		Email:    user.Email,
		Role:     user.Role,
		DriverID: driverID,
	}
}

// auditActions returns every audited action for one entity, oldest first.
func (e *env) auditActions(entity string, entityID int64) []string {
	e.t.Helper()
	rows, err := e.pool.Query(context.Background(),
		`SELECT action FROM audit_log WHERE entity = $1 AND entity_id = $2 ORDER BY id`, entity, entityID)
	if err != nil {
		e.t.Fatalf("reading audit log: %v", err)
	}
	defer rows.Close()

	var actions []string
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			e.t.Fatalf("scanning audit row: %v", err)
		}
		actions = append(actions, action)
	}
	return actions
}
