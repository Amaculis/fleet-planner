import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

// TimelineWeekView.vue renders a multi-day trip as a single ".tl-span-bar" spanning
// every day column it touches, instead of repeating a chip per day (see that
// component's own comments — this was a real, previously screenshot-confirmed bug
// class: bars rendering out of bounds / taller than their row). No e2e test had ever
// actually created a multi-day trip and looked for the resulting bar.
test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

test("a multi-day trip renders as one spanning bar in the week timeline", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const destination = `MultiDayDest${suffix}`;

  await page.goto("/app/");
  const setup = await page.evaluate(
    async ({ destination, suffix }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      // A dedicated, freshly created bus and driver (rather than reusing whatever the
      // fleet already has) — reused buses/drivers can already be booked somewhere in
      // this window by other e2e specs' own near-"now" trips, and this test only cares
      // about the rendering, not about finding a genuinely free existing vehicle.
      const busResp = await fetch("/api/buses", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ plate: `MD${suffix}`, model: "Test Model", seats: 40, status: "active" }),
      });
      const bus = await busResp.json();
      const driverResp = await fetch("/api/drivers", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ fullName: `MultiDay Driver ${suffix}`, licenseExpiry: "", isActive: true }),
      });
      const driver = await driverResp.json();

      // Starts this morning, ends two calendar days later — guaranteed multi-day
      // regardless of what time "now" is when this runs.
      const now = new Date();
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 8, 0);
      const end = new Date(start.getTime() + 2 * 24 * 60 * 60 * 1000);
      const pad = (n) => String(n).padStart(2, "0");
      const fmt = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
      const tripResp = await fetch("/api/trips", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ origin: "MultiDayOrigin", destination, scheduledStart: fmt(start), scheduledEnd: fmt(end), notes: null }),
      });
      const trip = await tripResp.json();
      const assignResp = await fetch(`/api/trips/${trip.id}/assign`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ busId: bus.id, driverId: driver.id }),
      });
      return { assigned: assignResp.status === 200, tripId: trip.id };
    },
    { destination, suffix }
  );
  expect(setup.assigned, "setting up an assigned multi-day trip").toBe(true);

  const errors = trackConsoleErrors(page);
  await page.goto("/app/timeline");
  await page.getByRole("tab", { name: /^week$/i }).click();
  await page.waitForTimeout(500);

  const spanBar = page.locator(".tl-span-bar", { hasText: destination });
  await expect(spanBar).toBeVisible();

  // A span bar covers 2+ day columns, so it must be noticeably wider than a
  // single-day chip in the same row — the concrete geometry check for "spans
  // multiple days" rather than "rendered as text somewhere on the page".
  const chip = page.locator(".tl-chip").first();
  if (await chip.count()) {
    const [barBox, chipBox] = await Promise.all([spanBar.boundingBox(), chip.boundingBox()]);
    expect(barBox.width).toBeGreaterThan(chipBox.width * 1.5);
  }

  await spanBar.click();
  await expect(page).toHaveURL(new RegExp(`/app/trips/${setup.tripId}$`));

  expect(errors, `console errors on the week timeline:\n${errors.join("\n")}`).toEqual([]);
});
