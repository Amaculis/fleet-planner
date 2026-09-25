import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

// MyTrips.vue is driver-only (RequireRole(RoleDriver) — see server.go), so the admin
// session every other test reuses via storageState can't exercise it. This test
// starts as the admin (the project's default storageState), uses that session to
// create a throwaway driver account via the API, then switches to it.
//
// Setup calls go through page.evaluate(fetch(...)) rather than Playwright's
// standalone `request` fixture — that fixture is a plain Node HTTP client with no
// route to the container network at all, so it can't reach the domain
// host-resolver-rules maps to the caddy container (only Chromium's own network stack
// benefits from that mapping). Found the same way in smoke.spec.js's CSP test.
test("driver: my trips loads with no console errors", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const driverName = `E2E Driver ${suffix}`;
  const email = `e2e-driver-${suffix}@example.com`;
  const password = "correct-horse-battery-staple";

  await page.goto("/app/");
  const result = await page.evaluate(
    async ({ driverName, email, password }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const driverResp = await fetch("/api/drivers", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ fullName: driverName, licenseExpiry: "", isActive: true }),
      });
      const driver = await driverResp.json();
      const userResp = await fetch("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ email, password, role: "driver", driverId: driver.id }),
      });
      return { driverStatus: driverResp.status, userStatus: userResp.status, driver };
    },
    { driverName, email, password }
  );
  expect(result.driverStatus, "creating the test driver").toBe(201);
  expect(result.userStatus, "creating the test driver's login").toBe(201);

  // Switch from the admin session to the new driver's.
  await page.context().clearCookies();
  const errors = trackConsoleErrors(page);
  await page.goto("/app/login");
  await page.getByRole("textbox", { name: /email/i }).fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(/\/app\/(dashboard)?$/);

  await page.goto("/app/my/trips");
  await expect(page.getByText(/my trips|mani reisi|мои рейсы/i).first()).toBeVisible();
  expect(errors, `console errors on my/trips:\n${errors.join("\n")}`).toEqual([]);
});

test("driver: start and finish a trip from my trips", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const driverName = `E2E Driver ${suffix}`;
  const email = `e2e-driver-${suffix}@example.com`;
  const password = "correct-horse-battery-staple";

  await page.goto("/app/");
  const setup = await page.evaluate(
    async ({ driverName, email, password, suffix }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const driverResp = await fetch("/api/drivers", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ fullName: driverName, licenseExpiry: "", isActive: true }),
      });
      const driver = await driverResp.json();
      await fetch("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ email, password, role: "driver", driverId: driver.id }),
      });
      const busesResp = await fetch("/api/buses?assignable=1").then((r) => r.json());
      // Trip must be in its scheduled window (not far in the future) since the driver's
      // start/finish actions are for a trip that's actually happening now.
      const start = new Date(Date.now() - 5 * 60 * 1000);
      const end = new Date(Date.now() + 60 * 60 * 1000);
      const pad = (n) => String(n).padStart(2, "0");
      const fmt = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
      const tripResp = await fetch("/api/trips", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ origin: `StartFinishOrigin${suffix}`, destination: "Dest", scheduledStart: fmt(start), scheduledEnd: fmt(end), notes: null }),
      });
      const trip = await tripResp.json();
      let assigned = false;
      for (const bus of busesResp) {
        const r = await fetch(`/api/trips/${trip.id}/assign`, {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
          body: JSON.stringify({ busId: bus.id, driverId: driver.id }),
        });
        if (r.status === 200) {
          assigned = true;
          break;
        }
      }
      return { assigned };
    },
    { driverName, email, password, suffix }
  );
  expect(setup.assigned, "setting up a planned trip for the test driver").toBe(true);

  await page.context().clearCookies();
  const errors = trackConsoleErrors(page);
  await page.goto("/app/login");
  await page.getByRole("textbox", { name: /email/i }).fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(/\/app\/(dashboard)?$/);

  await page.goto("/app/my/trips");
  await page.waitForTimeout(500);

  await expect(page.getByText(`StartFinishOrigin${suffix}`)).toBeVisible();
  await page.getByRole("button", { name: /^start trip$/i }).click();
  await expect(page.getByText(/^in progress$/i)).toBeVisible();

  await page.getByRole("button", { name: /^finish trip$/i }).click();
  await expect(page.getByText(/^completed$/i)).toBeVisible();
  await expect(page.getByRole("button", { name: /^start trip$/i })).not.toBeVisible();
  await expect(page.getByRole("button", { name: /^finish trip$/i })).not.toBeVisible();

  expect(errors, `console errors starting/finishing a trip:\n${errors.join("\n")}`).toEqual([]);
});
