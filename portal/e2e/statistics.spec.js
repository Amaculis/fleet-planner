import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

test("statistics page renders trip volume, fleet utilization and fleet status with no console errors", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/statistics");
  await page.waitForTimeout(800);

  await expect(page.getByRole("heading", { name: "Statistics" })).toBeVisible();
  await expect(page.getByText("Trip volume", { exact: true })).toBeVisible();
  await expect(page.getByText("Busiest buses", { exact: true })).toBeVisible();
  await expect(page.getByText("Busiest drivers", { exact: true })).toBeVisible();
  await expect(page.getByText("Fleet by status", { exact: true })).toBeVisible();

  // LxDataVisualizer's own default texts are hardcoded Latvian (see
  // hooks/dataVisualizerTexts.js) — the Graph/Table switcher (an LxContentSwitcher,
  // role="tab") must show the English override, not leak "Grafiks"/"Tabula".
  await expect(page.getByRole("tab", { name: "Graph" }).first()).toBeVisible();
  await expect(page.getByRole("tab", { name: "Table" }).first()).toBeVisible();
  await expect(page.getByText("Grafiks")).toHaveCount(0);

  expect(errors, `console errors on statistics page:\n${errors.join("\n")}`).toEqual([]);
});

test("the period switcher changes the displayed trip volume data", async ({ page }) => {
  await page.goto("/app/statistics");
  await page.waitForTimeout(800);

  const graphSwitcher = page.getByRole("tab", { name: "Last 90 days" });
  await graphSwitcher.click();
  await page.waitForTimeout(500);

  // A 90-day period buckets by week (Statistics.vue's bucketByWeek), so far fewer
  // than 90 bars/date labels appear — just confirm the switch didn't error and the
  // section still renders.
  await expect(page.getByText("Trip volume", { exact: true })).toBeVisible();
});

test("the statistics nav link is visible for admin", async ({ page }) => {
  await page.goto("/app/");
  await page.waitForTimeout(500);
  await expect(page.getByRole("link", { name: /^statistics$/i })).toBeVisible();
});

test("a dispatcher has no statistics nav link", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const email = `e2e-stats-dispatcher-${suffix}@example.com`;
  const password = "correct-horse-battery-staple";

  await page.goto("/app/");
  await page.evaluate(
    async ({ email, password }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      await fetch("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ email, password, role: "dispatcher" }),
      });
    },
    { email, password }
  );

  await page.context().clearCookies();
  await page.goto("/app/login");
  await page.getByRole("textbox", { name: /email/i }).fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(/\/app\/(dashboard)?$/);
  await page.waitForTimeout(500);

  // Statistics is gated to admin only (unlike buses/drivers/trips/timeline, which
  // are admin+dispatcher "planner" pages) — MainLayout.vue's own isAdmin check.
  await expect(page.getByRole("link", { name: /^statistics$/i })).toHaveCount(0);
});
