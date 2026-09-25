import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

// Same "DD.MM.YYYY. HH:MM" the picker itself shows and parses (see crud.spec.js).
const fmt = (d) =>
  `${String(d.getDate()).padStart(2, "0")}.${String(d.getMonth() + 1).padStart(2, "0")}.${d.getFullYear()}. ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;

async function fillDate(page, label, date) {
  await page.getByLabel(label).fill(fmt(date));
  await page.keyboard.press("Tab"); // the picker only commits its value on blur
}

test("the trip form shows both footer actions, English required markers, and no console errors", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/trips/new");
  await page.waitForTimeout(500);

  await expect(page.getByRole("button", { name: /^save/i })).toBeVisible();
  // Cancel must render: LxForm silently drops footer actions whose kind isn't
  // primary/secondary/tertiary/additional, and a "ghost" Cancel once vanished this way.
  await expect(page.getByRole("button", { name: /^cancel/i })).toBeVisible();
  // LxForm's built-in texts are Latvian unless overridden.
  await expect(page.getByText(/obligāts|neobligāts|Citas darbības/)).toHaveCount(0);

  expect(errors, `console errors on the empty trip form:\n${errors.join("\n")}`).toEqual([]);
});

test("saving an empty trip form flags each required field inline and stays on the form", async ({ page }) => {
  await page.goto("/app/trips/new");
  await page.waitForTimeout(500);
  await expect(page.getByText("This field is required.")).toHaveCount(0); // nothing shown before Save

  await page.getByRole("button", { name: /^save/i }).click();
  await expect(page.getByText("This field is required.")).toHaveCount(4); // origin, destination, start, end
  await expect(page).toHaveURL(/\/app\/trips\/new$/);

  // ...and the messages clear as the fields are fixed.
  await page.getByLabel(/^origin/i).fill("Rīga");
  await expect(page.getByText("This field is required.")).toHaveCount(3);
});

test("an end before the start is rejected next to the end field, and clears once fixed", async ({ page }) => {
  await page.goto("/app/trips/new");
  await page.waitForTimeout(500);
  await page.getByLabel(/^origin/i).fill("Rīga");
  await page.getByLabel(/^destination/i).fill("Cēsis");

  const start = new Date(Date.now() + 48 * 60 * 60 * 1000);
  await fillDate(page, /scheduled start/i, start);
  await fillDate(page, /scheduled end/i, new Date(start.getTime() - 60 * 60 * 1000));
  await page.getByRole("button", { name: /^save/i }).click();

  await expect(page.getByText("The end must be after the start.")).toBeVisible();
  await expect(page).toHaveURL(/\/app\/trips\/new$/);

  await fillDate(page, /scheduled end/i, new Date(start.getTime() + 2 * 60 * 60 * 1000));
  await expect(page.getByText("The end must be after the start.")).toHaveCount(0);
});

test("Cancel returns to the trip being edited, or to the list from a new trip", async ({ page }) => {
  await page.goto("/app/");
  const trip = await page.evaluate(async () => {
    const me = await fetch("/api/auth/me").then((r) => r.json());
    const pad = (n) => String(n).padStart(2, "0");
    const f = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
    const start = new Date(Date.now() + 3000 * 60 * 60 * 1000);
    const resp = await fetch("/api/trips", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
      body: JSON.stringify({ origin: "CancelFrom", destination: "CancelTo", scheduledStart: f(start), scheduledEnd: f(new Date(start.getTime() + 3600000)), notes: null }),
    });
    return resp.json();
  });

  await page.goto(`/app/trips/${trip.id}/edit`);
  await page.getByRole("button", { name: /^cancel/i }).click();
  await expect(page).toHaveURL(new RegExp(`/app/trips/${trip.id}$`));

  await page.goto("/app/trips/new");
  await page.getByRole("button", { name: /^cancel/i }).click();
  await expect(page).toHaveURL(/\/app\/trips$/);
});
