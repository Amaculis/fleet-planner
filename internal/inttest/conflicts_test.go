package inttest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

// TestAssignmentConflicts is the core invariant: a bus or a driver is never on two
// overlapping trips. Each case goes through the service, which pre-checks, and through
// Postgres, which enforces.
func TestAssignmentConflicts(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		// firstTrip and secondTrip are [startHour, endHour) windows on the same day.
		first, second  [2]int
		sameBus        bool
		sameDriver     bool
		cancelFirst    bool
		wantConflict   bool
		wantSubjectKey string
	}{
		{
			name:  "same bus, overlapping windows, is refused",
			first: [2]int{8, 12}, second: [2]int{10, 14},
			sameBus: true, wantConflict: true, wantSubjectKey: "bus",
		},
		{
			name:  "same driver, overlapping windows, is refused",
			first: [2]int{8, 12}, second: [2]int{10, 14},
			sameDriver: true, wantConflict: true, wantSubjectKey: "driver",
		},
		{
			name: "same bus, back-to-back windows, is allowed",
			// The ranges are half-open: 12:00 belongs to the second trip only.
			first: [2]int{8, 12}, second: [2]int{12, 16},
			sameBus: true, sameDriver: true,
		},
		{
			name:  "same bus, second trip entirely inside the first, is refused",
			first: [2]int{8, 18}, second: [2]int{10, 12},
			sameBus: true, wantConflict: true, wantSubjectKey: "bus",
		},
		{
			name:  "same bus, first trip entirely inside the second, is refused",
			first: [2]int{10, 12}, second: [2]int{8, 18},
			sameBus: true, wantConflict: true, wantSubjectKey: "bus",
		},
		{
			name:  "same bus, windows touching by one hour, is refused",
			first: [2]int{8, 12}, second: [2]int{11, 13},
			sameBus: true, wantConflict: true, wantSubjectKey: "bus",
		},
		{
			name:  "different bus and driver, overlapping windows, is allowed",
			first: [2]int{8, 12}, second: [2]int{8, 12},
		},
		{
			name:  "a cancelled trip releases its bus and driver",
			first: [2]int{8, 12}, second: [2]int{10, 14},
			sameBus: true, sameDriver: true, cancelFirst: true,
		},
		{
			name:  "same bus, no overlap at all, is allowed",
			first: [2]int{8, 10}, second: [2]int{14, 16},
			sameBus: true, sameDriver: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			admin := e.newAdmin("admin@example.lv", "correct-horse-battery-staple")
			actor := e.identityFor(admin, nil)

			busA := e.newBus("AA-1111")
			busB := e.newBus("BB-2222")
			driverA := e.newDriver("Driver A")
			driverB := e.newDriver("Driver B")

			tripOne := e.newTrip("Riga", "Liepaja", tt.first[0], tt.first[1])
			tripTwo := e.newTrip("Riga", "Ventspils", tt.second[0], tt.second[1])

			if _, err := e.assignments.Assign(ctx, actor, tripOne.ID, busA.ID, driverA.ID, service.Meta{}); err != nil {
				t.Fatalf("first assignment should succeed: %v", err)
			}

			if tt.cancelFirst {
				if _, err := e.trips.SetStatus(ctx, actor, tripOne.ID, domain.TripCancelled, service.Meta{}); err != nil {
					t.Fatalf("cancelling the first trip: %v", err)
				}
			}

			secondBus, secondDriver := busB.ID, driverB.ID
			if tt.sameBus {
				secondBus = busA.ID
			}
			if tt.sameDriver {
				secondDriver = driverA.ID
			}

			_, err := e.assignments.Assign(ctx, actor, tripTwo.ID, secondBus, secondDriver, service.Meta{})

			if !tt.wantConflict {
				if err != nil {
					t.Fatalf("second assignment should have been allowed, got: %v", err)
				}
				return
			}

			if !errors.Is(err, domain.ErrTimeConflict) {
				t.Fatalf("expected a time conflict, got: %v", err)
			}
			var conflict *service.ConflictError
			if !errors.As(err, &conflict) {
				t.Fatalf("expected a *service.ConflictError carrying the blocking trips, got: %v", err)
			}
			if conflict.Subject != tt.wantSubjectKey {
				t.Errorf("conflict subject = %q, want %q", conflict.Subject, tt.wantSubjectKey)
			}
			if len(conflict.Conflicts) == 0 {
				t.Error("the conflict carries no overlapping trips to show the user")
			} else if conflict.Conflicts[0].TripID != tripOne.ID {
				t.Errorf("blocking trip = %d, want %d", conflict.Conflicts[0].TripID, tripOne.ID)
			}
		})
	}
}

// TestRescheduleIntoConflict covers the other way a double-booking could appear: not by
// assigning, but by moving an already-assigned trip on top of another one.
func TestRescheduleIntoConflict(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", "correct-horse-battery-staple")
	actor := e.identityFor(admin, nil)

	bus := e.newBus("AA-1111")
	driver := e.newDriver("Driver A")

	morning := e.newTrip("Riga", "Liepaja", 8, 12)
	evening := e.newTrip("Liepaja", "Riga", 18, 22)

	if _, err := e.assignments.Assign(ctx, actor, morning.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning the morning trip: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, actor, evening.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning the evening trip: %v", err)
	}

	// Move the evening trip on top of the morning one.
	clashing := evening
	clashing.ScheduledStart = at(10)
	clashing.ScheduledEnd = at(14)

	_, err := e.trips.Update(ctx, actor, clashing, service.Meta{})
	if !errors.Is(err, domain.ErrTimeConflict) {
		t.Fatalf("rescheduling into a clash should be refused, got: %v", err)
	}

	// The trip must be unchanged: the whole update rolled back.
	after, err := e.trips.Get(ctx, actor, evening.ID)
	if err != nil {
		t.Fatalf("reading the trip back: %v", err)
	}
	if !after.ScheduledStart.Equal(evening.ScheduledStart) {
		t.Errorf("trip start = %v, want unchanged %v", after.ScheduledStart, evening.ScheduledStart)
	}

	// Moving it somewhere free still works.
	moved := evening
	moved.ScheduledStart = at(13)
	moved.ScheduledEnd = at(17)
	if _, err := e.trips.Update(ctx, actor, moved, service.Meta{}); err != nil {
		t.Fatalf("moving the trip to a free slot: %v", err)
	}
}

// TestConcurrentAssignmentsRaceOnce proves the guarantee does not depend on the service
// pre-check. Both goroutines read the same empty schedule and both try to book the same
// bus; only the EXCLUDE constraint stands between them and a double booking.
func TestConcurrentAssignmentsRaceOnce(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", "correct-horse-battery-staple")
	actor := e.identityFor(admin, nil)

	bus := e.newBus("AA-1111")
	driverA := e.newDriver("Driver A")
	driverB := e.newDriver("Driver B")

	tripOne := e.newTrip("Riga", "Liepaja", 8, 12)
	tripTwo := e.newTrip("Riga", "Ventspils", 9, 13) // overlaps tripOne

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		succeed int
		failed  int
		lastErr error
	)
	start := make(chan struct{})

	for _, trip := range []struct {
		id       int64
		driverID int64
	}{{tripOne.ID, driverA.ID}, {tripTwo.ID, driverB.ID}} {
		wg.Add(1)
		go func(tripID, driverID int64) {
			defer wg.Done()
			<-start // line both goroutines up on the same starting gun
			_, err := e.assignments.Assign(ctx, actor, tripID, bus.ID, driverID, service.Meta{})
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				succeed++
			} else {
				failed++
				lastErr = err
			}
		}(trip.id, trip.driverID)
	}
	close(start)
	wg.Wait()

	if succeed != 1 || failed != 1 {
		t.Fatalf("expected exactly one booking to win, got %d successes and %d failures (last error: %v)",
			succeed, failed, lastErr)
	}
	if !errors.Is(lastErr, domain.ErrTimeConflict) {
		t.Errorf("the losing request should report a time conflict, got: %v", lastErr)
	}

	var booked int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM assignments WHERE bus_id = $1`, bus.ID).Scan(&booked); err != nil {
		t.Fatalf("counting assignments: %v", err)
	}
	if booked != 1 {
		t.Fatalf("bus is booked %d times for overlapping trips; the EXCLUDE constraint did not hold", booked)
	}
}

// TestCompletedTripKeepsItsDriver protects the payroll seam: once a trip is completed,
// the link between the driver and the worked time cannot be deleted or re-pointed.
func TestCompletedTripKeepsItsDriver(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", "correct-horse-battery-staple")
	actor := e.identityFor(admin, nil)

	bus := e.newBus("AA-1111")
	otherBus := e.newBus("BB-2222")
	driver := e.newDriver("Driver A")
	otherDriver := e.newDriver("Driver B")
	trip := e.newTrip("Riga", "Liepaja", 8, 12)

	if _, err := e.assignments.Assign(ctx, actor, trip.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}
	for _, status := range []domain.TripStatus{domain.TripInProgress, domain.TripCompleted} {
		if _, err := e.trips.SetStatus(ctx, actor, trip.ID, status, service.Meta{}); err != nil {
			t.Fatalf("moving trip to %s: %v", status, err)
		}
	}

	if err := e.assignments.Unassign(ctx, actor, trip.ID, service.Meta{}); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("unassigning a completed trip should be refused, got: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, otherBus.ID, otherDriver.ID, service.Meta{}); !errors.Is(err, domain.ErrConflict) {
		t.Errorf("re-assigning a completed trip should be refused, got: %v", err)
	}

	assignment, err := e.assignments.GetForTrip(ctx, actor, trip.ID)
	if err != nil {
		t.Fatalf("reading the assignment: %v", err)
	}
	if assignment.DriverID != driver.ID || assignment.BusID != bus.ID {
		t.Fatalf("the completed trip's assignment changed: bus %d driver %d", assignment.BusID, assignment.DriverID)
	}

	// Reopening is the sanctioned way to fix a mistake, and it is audited.
	if _, err := e.trips.SetStatus(ctx, actor, trip.ID, domain.TripInProgress, service.Meta{}); err != nil {
		t.Fatalf("reopening the trip: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, otherBus.ID, otherDriver.ID, service.Meta{}); err != nil {
		t.Fatalf("re-assigning after reopening: %v", err)
	}
}

// TestMutationsAreAudited checks that the audit trail actually records the work.
func TestMutationsAreAudited(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", "correct-horse-battery-staple")
	actor := e.identityFor(admin, nil)

	trip, err := e.trips.Create(ctx, actor, domain.Trip{
		Origin: "Riga", Destination: "Liepaja",
		ScheduledStart: at(8), ScheduledEnd: at(12),
	}, service.Meta{RequestID: "test"})
	if err != nil {
		t.Fatalf("creating trip: %v", err)
	}
	updated := trip
	updated.Destination = "Ventspils"
	if _, err := e.trips.Update(ctx, actor, updated, service.Meta{RequestID: "test"}); err != nil {
		t.Fatalf("updating trip: %v", err)
	}
	if _, err := e.trips.SetStatus(ctx, actor, trip.ID, domain.TripCancelled, service.Meta{RequestID: "test"}); err != nil {
		t.Fatalf("cancelling trip: %v", err)
	}

	got := e.auditActions("trip", trip.ID)
	want := []string{"create", "update", "status"}
	if len(got) != len(want) {
		t.Fatalf("audit actions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("audit actions = %v, want %v", got, want)
		}
	}

	// A refused mutation must leave no audit entry: the whole transaction rolls back.
	var before, after int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log`).Scan(&before); err != nil {
		t.Fatalf("counting audit rows: %v", err)
	}
	bad := trip
	bad.Origin = "" // fails validation
	if _, err := e.trips.Update(ctx, actor, bad, service.Meta{}); err == nil {
		t.Fatal("an invalid update should have been refused")
	}
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log`).Scan(&after); err != nil {
		t.Fatalf("counting audit rows: %v", err)
	}
	if after != before {
		t.Errorf("a refused mutation wrote %d audit row(s)", after-before)
	}
}
