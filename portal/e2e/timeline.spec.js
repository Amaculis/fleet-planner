import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

test("timeline loads with no console errors and shows nav/day controls", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/timeline");
  await expect(page.getByText(/timeline|laika grafiks|график/i).first()).toBeVisible();
  await expect(page.getByRole("button", { name: /previous day/i })).toBeVisible();
  await expect(page.getByRole("button", { name: /next day/i })).toBeVisible();
  await expect(page.getByRole("button", { name: /^today$/i })).toBeVisible();
  expect(errors, `console errors on timeline:\n${errors.join("\n")}`).toEqual([]);
});

test("timeline day navigation works without erroring", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/timeline");
  await page.getByRole("button", { name: /next day/i }).click();
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: /previous day/i }).click();
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: /^today$/i }).click();
  await page.waitForTimeout(500);
  expect(errors, `console errors navigating the timeline:\n${errors.join("\n")}`).toEqual([]);
});

test("timeline switches between day, week and month views without erroring", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/timeline");

  await page.getByRole("tab", { name: /^week$/i }).click();
  await page.waitForTimeout(500);
  await expect(page.getByRole("button", { name: /previous week/i })).toBeVisible();
  await page.getByRole("button", { name: /next week/i }).click();
  await page.waitForTimeout(500);

  await page.getByRole("tab", { name: /^month$/i }).click();
  await page.waitForTimeout(500);
  await expect(page.getByRole("button", { name: /previous month/i })).toBeVisible();
  await page.getByRole("button", { name: /next month/i }).click();
  await page.waitForTimeout(500);

  await page.getByRole("tab", { name: /^day$/i }).click();
  await page.waitForTimeout(500);
  await expect(page.getByRole("button", { name: /previous day/i })).toBeVisible();

  expect(errors, `console errors switching timeline views:\n${errors.join("\n")}`).toEqual([]);
});

// Previous/next are icon-only LxButtons (no CSS override of LxButton's icon order);
// their label stays the accessible name and tooltip.
test("the previous/next buttons are icon-only with an accessible name", async ({ page }) => {
  await page.goto("/app/timeline");
  await page.waitForTimeout(500);
  for (const name of [/previous day/i, /next day/i]) {
    const button = page.getByRole("button", { name });
    await expect(button).toBeVisible();
    await expect(button).toHaveClass(/lx-button-icon-only/);
  }
});

// Jumping straight to a date, not only stepping with previous/next: the same picker
// drives every view (a picked date selects the day, or the week/month containing it).
test("the timeline date picker jumps directly to a day, week or month", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  const pick = async (view, typed) => {
    await page.goto(`/app/timeline?view=${view}`);
    await page.waitForTimeout(500);
    await page.locator(".timeline-date-field input").first().fill(typed);
    await page.keyboard.press("Tab");
    await page.waitForTimeout(700);
  };

  await pick("day", "15.10.2026.");
  await expect(page.locator(".timeline-date-field input").first()).toHaveValue("15.10.2026.");

  await pick("week", "15.10.2026."); // a Thursday: the Monday-first week is Oct 12-18
  await expect(page.locator(".timeline-range-label")).toContainText("Oct 12, 2026 – Oct 18, 2026");

  await pick("month", "15.11.2026.");
  await expect(page.locator(".timeline-range-label")).toContainText("November 2026");

  expect(errors, `console errors picking timeline dates:\n${errors.join("\n")}`).toEqual([]);
});
