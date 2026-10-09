import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

// Same "DD.MM.YYYY. HH:MM" / "DD.MM.YYYY." the pickers show and parse (see tripForm.spec.js).
const fmtDateTime = (d) =>
  `${String(d.getDate()).padStart(2, "0")}.${String(d.getMonth() + 1).padStart(2, "0")}.${d.getFullYear()}. ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
const fmtDate = (d) => `${String(d.getDate()).padStart(2, "0")}.${String(d.getMonth() + 1).padStart(2, "0")}.${d.getFullYear()}.`;

async function fillPicker(page, label, text) {
  await page.getByLabel(label).fill(text);
  await page.keyboard.press("Tab"); // the picker only commits its value on blur
}

// A Monday at least a week out, so the series' own weekday math has something to chew
// on regardless of what day this suite happens to run.
function nextMonday() {
  const d = new Date(Date.now() + 21 * 24 * 60 * 60 * 1000);
  const offset = (8 - d.getDay()) % 7 || 7;
  d.setDate(d.getDate() + offset);
  d.setHours(8, 0, 0, 0);
  return d;
}

test("creating a repeating trip generates one occurrence per matching weekday, with no console errors", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/trips/new");
  await page.waitForTimeout(500);

  const suffix = Date.now() % 100000;
  await page.getByLabel(/^origin/i).fill(`SeriesOrigin${suffix}`);
  await page.getByLabel(/^destination/i).fill(`SeriesDest${suffix}`);

  await page.locator("input.lx-toggle").dispatchEvent("click");
  await page.waitForTimeout(300);

  const start = nextMonday();
  const end = new Date(start.getTime() + 2 * 60 * 60 * 1000);
  const endsOn = new Date(start.getTime() + 20 * 24 * 60 * 60 * 1000); // ~3 Mondays later

  await fillPicker(page, "Scheduled start", fmtDateTime(start));
  await fillPicker(page, "Scheduled end", fmtDateTime(end));
  await page.getByText("Monday", { exact: true }).click();
  await fillPicker(page, "Repeat until", fmtDate(endsOn));

  const [resp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/api/trip-series") && r.request().method() === "POST"),
    page.getByRole("button", { name: /^save/i }).click(),
  ]);
  expect(resp.status()).toBe(201);
  const created = await resp.json();

  // Every Monday in a ~3-week window from the first one: exactly 3 occurrences, all
  // sharing one seriesId, none on a non-Monday.
  expect(created.length).toBe(3);
  const seriesId = created[0].seriesId;
  for (const trip of created) {
    expect(trip.seriesId).toBe(seriesId);
    expect(new Date(trip.scheduledStart).getDay()).toBe(1);
  }

  await expect(page).toHaveURL(/\/app\/trips$/);
  await expect(page.getByText(`${created.length} trips created.`)).toBeVisible();

  expect(errors, `console errors creating a trip series:\n${errors.join("\n")}`).toEqual([]);
});

async function createSeriesViaAPI(page, suffix) {
  return page.evaluate(async (suffix) => {
    const me = await fetch("/api/auth/me").then((r) => r.json());
    const pad = (n) => String(n).padStart(2, "0");
    const f = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
    const start = new Date(Date.now() + 21 * 24 * 60 * 60 * 1000);
    start.setDate(start.getDate() + ((8 - start.getDay()) % 7 || 7));
    start.setHours(9, 0, 0, 0);
    const end = new Date(start.getTime() + 60 * 60 * 1000);
    const endsOn = new Date(start.getTime() + 20 * 24 * 60 * 60 * 1000);
    const resp = await fetch("/api/trip-series", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
      body: JSON.stringify({
        origin: `ApiSeriesOrigin${suffix}`,
        destination: `ApiSeriesDest${suffix}`,
        daysOfWeek: [start.getDay() === 0 ? 7 : start.getDay()],
        firstStart: f(start),
        firstEnd: f(end),
        endsOn: `${endsOn.getFullYear()}-${pad(endsOn.getMonth() + 1)}-${pad(endsOn.getDate())}`,
        paymentStatus: "unpaid",
        notes: null,
      }),
    });
    return resp.json();
  }, suffix);
}

test("editing a series trip offers this-only vs this-and-future, and future applies to every later planned trip", async ({ page }) => {
  await page.goto("/app/");
  const suffix = Date.now() % 100000;
  const created = await createSeriesViaAPI(page, suffix);
  expect(created.length).toBe(3);

  await page.goto(`/app/trips/${created[0].id}/edit`);
  await page.waitForTimeout(500);
  await expect(page.getByText("Part of a repeating series")).toBeVisible();

  await page.getByLabel(/^destination/i).fill(`ApiSeriesDestEdited${suffix}`);
  await page.getByRole("button", { name: /^save/i }).click();
  await page.waitForTimeout(400);

  await expect(page.getByRole("button", { name: "This and future trips" })).toBeVisible();
  await expect(page.getByRole("button", { name: "This trip only" })).toBeVisible();
  await page.getByRole("button", { name: "This and future trips" }).click();
  await page.waitForTimeout(600);

  const after = await page.evaluate(
    (ids) => Promise.all(ids.map((id) => fetch(`/api/trips/${id}`).then((r) => r.json()))),
    created.map((t) => t.id)
  );
  for (const trip of after) {
    expect(trip.destination).toBe(`ApiSeriesDestEdited${suffix}`);
  }
});

test("cancelling a series trip with this-and-future leaves earlier occurrences planned", async ({ page }) => {
  await page.goto("/app/");
  const suffix = Date.now() % 100000;
  const created = await createSeriesViaAPI(page, suffix);
  expect(created.length).toBe(3);

  // Cancel from the second occurrence: the first should stay planned, the rest cancelled.
  await page.goto(`/app/trips/${created[1].id}`);
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: "Cancelled" }).click();
  await page.waitForTimeout(400);
  await page.getByRole("button", { name: "This and future trips" }).click();
  await page.waitForTimeout(600);

  const after = await page.evaluate(
    (ids) => Promise.all(ids.map((id) => fetch(`/api/trips/${id}`).then((r) => r.json()))),
    created.map((t) => t.id)
  );
  expect(after[0].status).toBe("planned");
  expect(after[1].status).toBe("cancelled");
  expect(after[2].status).toBe("cancelled");
});
