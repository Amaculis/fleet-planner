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
