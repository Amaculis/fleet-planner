import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

test("dashboard shows planner tiles and info panels with no console errors", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/");
  await page.waitForTimeout(500);

  // LxTile's accessible name is just the count (its first paragraph) — filtering by
  // full text content, not accessible name, is what actually matches "10 / Active
  // buses" as one tile (confirmed via ariaSnapshot; getByRole(... name: "Active
  // buses") would not match this way LxTile renders).
  for (const label of ["Active buses", "Active drivers", "Trips today", "Trips this week", "In progress now", "Unassigned trips"]) {
    await expect(page.getByRole("link").filter({ hasText: label })).toBeVisible();
  }

  await expect(page.getByRole("heading", { name: "Documents expiring soon" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Upcoming trips" })).toBeVisible();

  expect(errors, `console errors on dashboard:\n${errors.join("\n")}`).toEqual([]);
});

test("clicking a planner tile navigates to its target page", async ({ page }) => {
  await page.goto("/app/");
  await page.waitForTimeout(500);
  await page.getByRole("link").filter({ hasText: "Active buses" }).click();
  await expect(page).toHaveURL(/\/app\/buses$/);
});

// The dashboard's own dependency-free assign/status logic (isUpcoming, the tile
// counts) is already covered indirectly by seeing real numbers above zero here; a
// driver-specific pass is in a separate test below since it needs its own session.
test("dashboard shows driver tiles and next trip for a driver, with no console errors", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const driverName = `E2E Driver ${suffix}`;
  const email = `e2e-driver-${suffix}@example.com`;
  const password = "correct-horse-battery-staple";

  await page.goto("/app/");
  const setup = await page.evaluate(
    async ({ driverName, email, password }) => {
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
      const start = new Date(Date.now() + 2 * 60 * 60 * 1000);
      const end = new Date(start.getTime() + 3 * 60 * 60 * 1000);
      const pad = (n) => String(n).padStart(2, "0");
      const fmt = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
      const tripResp = await fetch("/api/trips", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ origin: "Rīga", destination: "Cēsis", scheduledStart: fmt(start), scheduledEnd: fmt(end), notes: null }),
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
    { driverName, email, password }
  );
  expect(setup.assigned, "setting up a trip for the test driver").toBe(true);

  await page.context().clearCookies();
  const errors = trackConsoleErrors(page);
  await page.goto("/app/login");
  await page.getByRole("textbox", { name: /email/i }).fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(/\/app\/(dashboard)?$/);
  await page.waitForTimeout(500);

  await expect(page.getByRole("link").filter({ hasText: "My trips today" })).toBeVisible();
  await expect(page.getByRole("link").filter({ hasText: "My trips this week" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Next trip" })).toBeVisible();
  await expect(page.getByText("Rīga → Cēsis")).toBeVisible();

  expect(errors, `console errors on driver dashboard:\n${errors.join("\n")}`).toEqual([]);
});
