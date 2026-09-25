import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

// Creates a trip via the API (not through the form) so each test controls its own
// exact time window — needed for the conflict test below, and just faster/more
// reliable than driving TripForm.vue's date picker for setup that isn't itself under
// test (see crud.spec.js for that).
async function createTrip(page, { origin, destination, startOffsetHours, durationHours }) {
  return page.evaluate(
    async ({ origin, destination, startOffsetHours, durationHours }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const start = new Date(Date.now() + startOffsetHours * 60 * 60 * 1000);
      const end = new Date(start.getTime() + durationHours * 60 * 60 * 1000);
      const pad = (n) => String(n).padStart(2, "0");
      const fmt = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
      const resp = await fetch("/api/trips", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ origin, destination, scheduledStart: fmt(start), scheduledEnd: fmt(end), notes: null }),
      });
      return resp.json();
    },
    { origin, destination, startOffsetHours, durationHours }
  );
}

async function assignTrip(page, tripId, busId, driverId) {
  return page.evaluate(
    async ({ tripId, busId, driverId }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const resp = await fetch(`/api/trips/${tripId}/assign`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ busId, driverId }),
      });
      return { status: resp.status, body: await resp.json() };
    },
    { tripId, busId, driverId }
  );
}

async function firstAssignableBusAndDriver(page) {
  return page.evaluate(async () => {
    const [buses, drivers] = await Promise.all([
      fetch("/api/buses?assignable=1").then((r) => r.json()),
      fetch("/api/drivers?assignable=1").then((r) => r.json()),
    ]);
    return { bus: buses[0], driver: drivers[0] };
  });
}

test("assign a bus and driver to an unassigned trip", async ({ page }) => {
  await page.goto("/app/");
  // Each test in this file uses a distinct, run-varying offset range (base + suffix) so
  // reruns never land on the same bus+driver window a previous run already booked —
  // a fixed offset made "booking trip A" (and this test's own assign) spuriously fail
  // with a real conflict on the second run, not a bug in the app itself.
  const suffix = Date.now() % 100000;
  const trip = await createTrip(page, { origin: `AssignOrigin${suffix}`, destination: "Dest", startOffsetHours: 1000 + (suffix % 500), durationHours: 2 });
  const { bus, driver } = await firstAssignableBusAndDriver(page);

  const errors = trackConsoleErrors(page);
  await page.goto(`/app/trips/${trip.id}`);
  await page.waitForTimeout(500);

  await page.getByRole("combobox", { name: "Bus" }).click();
  await page.getByRole("option", { name: bus.plate }).click();
  await page.getByRole("combobox", { name: "Driver" }).click();
  await page.getByRole("option", { name: driver.fullName }).click();
  await page.getByRole("button", { name: /^assign$/i }).click();

  await expect(page.getByText(bus.plate)).toBeVisible();
  await expect(page.getByText(driver.fullName)).toBeVisible();
  await expect(page.getByRole("button", { name: /unassign/i })).toBeVisible();
  expect(errors, `console errors assigning a trip:\n${errors.join("\n")}`).toEqual([]);
});

test("unassign a trip", async ({ page }) => {
  await page.goto("/app/");
  const suffix = Date.now() % 100000;
  const trip = await createTrip(page, { origin: `UnassignOrigin${suffix}`, destination: "Dest", startOffsetHours: 1500 + (suffix % 500), durationHours: 2 });
  const { bus, driver } = await firstAssignableBusAndDriver(page);
  const assign = await assignTrip(page, trip.id, bus.id, driver.id);
  expect(assign.status, "setting up the assignment to unassign").toBe(200);

  await page.goto(`/app/trips/${trip.id}`);
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: /unassign/i }).click();

  await expect(page.getByText("No bus or driver assigned yet.")).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Bus" })).toBeVisible();
});

test("trip status transitions: planned -> in progress -> completed", async ({ page }) => {
  await page.goto("/app/");
  const suffix = Date.now() % 100000;
  const trip = await createTrip(page, { origin: `StatusOrigin${suffix}`, destination: "Dest", startOffsetHours: 2000 + (suffix % 500), durationHours: 2 });

  await page.goto(`/app/trips/${trip.id}`);
  await page.waitForTimeout(500);
  await expect(page.getByText("Planned")).toBeVisible();

  await page.getByRole("button", { name: /^in progress$/i }).click();
  await expect(page.getByText("In progress")).toBeVisible();
  await expect(page.getByRole("button", { name: /^completed$/i })).toBeVisible();

  await page.getByRole("button", { name: /^completed$/i }).click();
  await expect(page.getByText("Completed")).toBeVisible();
});

test("assigning an already-booked bus is rejected with a conflict, not a silent double-booking", async ({ page }) => {
  await page.goto("/app/");
  const suffix = Date.now() % 100000;
  const { bus, driver } = await firstAssignableBusAndDriver(page);
  // Offset varies per run (unlike the fixed offsets in the other tests above) so this
  // test's own trip A/B window never collides with leftover assignments a previous run
  // of this same test left on this bus+driver — that collision, not a real app bug,
  // is what made "booking trip A" itself return 409 on a rerun.
  const baseOffset = 300 + (suffix % 500);

  // Trip A: booked to bus/driver first.
  const tripA = await createTrip(page, { origin: `ConflictA${suffix}`, destination: "Dest", startOffsetHours: baseOffset, durationHours: 3 });
  const assignA = await assignTrip(page, tripA.id, bus.id, driver.id);
  expect(assignA.status, "booking trip A").toBe(200);

  // Trip B: overlaps trip A's window by an hour, same bus+driver — must be rejected.
  const tripB = await createTrip(page, { origin: `ConflictB${suffix}`, destination: "Dest", startOffsetHours: baseOffset + 2, durationHours: 3 });

  await page.goto(`/app/trips/${tripB.id}`);
  await page.waitForTimeout(500);
  await page.getByRole("combobox", { name: "Bus" }).click();
  await page.getByRole("option", { name: bus.plate }).click();
  await page.getByRole("combobox", { name: "Driver" }).click();
  await page.getByRole("option", { name: driver.fullName }).click();
  await page.getByRole("button", { name: /^assign$/i }).click();

  await expect(page.getByText(/already booked/i)).toBeVisible();
  // Trip B must still show as unassigned — the conflict must not have gone through.
  await expect(page.getByText("No bus or driver assigned yet.")).toBeVisible();
});
