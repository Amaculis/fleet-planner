import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

test("trip payment status is settable and shown on the list/detail views", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const origin = `PaymentOrigin${suffix}`;

  const errors = trackConsoleErrors(page);
  await page.goto("/app/trips/new");
  await page.getByLabel(/^origin/i).fill(origin);
  await page.getByLabel(/^destination/i).fill("PaymentDest");

  const start = new Date(Date.now() + 400 * 60 * 60 * 1000);
  const end = new Date(start.getTime() + 2 * 60 * 60 * 1000);
  const fmt = (d) =>
    `${String(d.getDate()).padStart(2, "0")}.${String(d.getMonth() + 1).padStart(2, "0")}.${d.getFullYear()}. ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  await page.getByLabel(/scheduled start/i).fill(fmt(start));
  await page.keyboard.press("Tab");
  await page.getByLabel(/scheduled end/i).fill(fmt(end));
  await page.keyboard.press("Tab");

  await page.getByRole("combobox", { name: "Payment status" }).click();
  await page.getByRole("option", { name: "Advance paid" }).click();
  await page.getByRole("button", { name: /save/i }).click();

  await expect(page).toHaveURL(/\/app\/trips\/\d+$/, { timeout: 10000 });
  await expect(page.getByText("Advance paid")).toBeVisible();
  const tripId = page.url().match(/\/trips\/(\d+)$/)[1];

  // TripList.vue has no search box (unlike Driver/User lists) — with many accumulated
  // trips from other e2e specs, LxDataGrid's virtualization can put this exact row
  // outside the rendered window (see crud.spec.js's own comments on this). Checking
  // the persisted value via the API is the reliable equivalent of "the list reflects
  // it", without depending on which rows happen to be scrolled into view.
  const persisted = await page.evaluate((id) => fetch(`/api/trips/${id}`).then((r) => r.json()), tripId);
  expect(persisted.paymentStatus).toBe("advance_paid");

  expect(errors, `console errors setting a trip's payment status:\n${errors.join("\n")}`).toEqual([]);
});

test("a driver's my-trips view never shows a payment status", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const email = `e2e-payment-driver-${suffix}@example.com`;
  const password = "correct-horse-battery-staple";

  await page.goto("/app/");
  const setup = await page.evaluate(
    async ({ email, password, suffix }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const driverResp = await fetch("/api/drivers", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ fullName: `Payment Driver ${suffix}`, licenseExpiry: "", isActive: true }),
      });
      const driver = await driverResp.json();
      await fetch("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ email, password, role: "driver", driverId: driver.id }),
      });
      const busesResp = await fetch("/api/buses?assignable=1").then((r) => r.json());
      const start = new Date(Date.now() + 410 * 60 * 60 * 1000);
      const end = new Date(start.getTime() + 2 * 60 * 60 * 1000);
      const pad = (n) => String(n).padStart(2, "0");
      const fmt = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
      const tripResp = await fetch("/api/trips", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({
          origin: "DriverViewCheck",
          destination: "Dest",
          scheduledStart: fmt(start),
          scheduledEnd: fmt(end),
          paymentStatus: "paid",
          notes: null,
        }),
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
    { email, password, suffix }
  );
  expect(setup.assigned, "setting up a paid trip for the test driver").toBe(true);

  await page.context().clearCookies();
  await page.goto("/app/login");
  await page.getByRole("textbox", { name: /email/i }).fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(/\/app\/(dashboard)?$/);

  // The API response itself must have no paymentStatus field — the RBAC boundary is
  // server-side, not just the frontend not rendering it.
  const body = await page.evaluate(() => fetch("/api/my/trips").then((r) => r.json()));
  for (const trip of body) {
    expect(Object.keys(trip)).not.toContain("paymentStatus");
  }

  await page.goto("/app/my/trips");
  await page.waitForTimeout(500);
  await expect(page.getByText(/paid|unpaid|reserved|advance/i)).toHaveCount(0);
});
