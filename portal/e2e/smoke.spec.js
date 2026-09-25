import { test, expect } from "@playwright/test";
import { trackConsoleErrors, login, ADMIN_EMAIL, ADMIN_PASSWORD } from "./support.js";

test.skip(!ADMIN_EMAIL || !ADMIN_PASSWORD, "E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD not set");

// The only test that logs in itself — every other test reuses the session
// e2e/auth.setup.js already established (see playwright.config.mjs's storageState).
// Starts from a clean, anonymous browser state so it actually exercises the login
// flow rather than finding itself already signed in.
test.describe("login", () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test("login works and reaches the dashboard with no console errors", async ({ page }) => {
    const errors = trackConsoleErrors(page);
    await login(page, ADMIN_EMAIL, ADMIN_PASSWORD);
    // Disambiguate: the shell's user menu AND the dashboard's greeting heading both
    // show the email once tiles/greeting were added to Dashboard.vue.
    await expect(page.getByRole("heading", { name: new RegExp(ADMIN_EMAIL) })).toBeVisible();
    expect(errors, `console errors on login:\n${errors.join("\n")}`).toEqual([]);
  });
});

// Every page a signed-in admin can reach, and text each one is expected to show —
// the same route list server.go's RBAC groups define. Already authenticated via
// storageState, so this is purely "does the page render cleanly."
const PAGES = [
  // Dashboard's own path is "" under the /app/ root (see router/routes.js), so its
  // real URL is /app/, not /app/dashboard — found via this test itself 404ing.
  { path: "/app/", text: "Dashboard" },
  { path: "/app/buses", text: "Buses" },
  { path: "/app/drivers", text: "Drivers" },
  { path: "/app/trips", text: "Trips" },
  { path: "/app/users", text: "Users" },
  { path: "/app/accessibility", text: "Accessibility settings" },
];

for (const { path, text } of PAGES) {
  test(`${path} loads with no console errors`, async ({ page }) => {
    const errors = trackConsoleErrors(page);
    await page.goto(path);
    await expect(page.getByText(text).first()).toBeVisible();
    expect(errors, `console errors on ${path}:\n${errors.join("\n")}`).toEqual([]);
  });
}

// Locks in the LxDataGrid texts fix: its default row-count footer is hardcoded
// Latvian ("1 ieraksts") unless the app supplies its own texts prop (see
// src/hooks/dataGridTexts.js). In English mode, that Latvian word must never appear.
test("the users list shows an English item count, not LxDataGrid's Latvian default", async ({ page }) => {
  await page.goto("/app/users");
  await expect(page.getByText(/\d+ item/)).toBeVisible();
  await expect(page.getByText("ieraksts")).toHaveCount(0);
});

// Locks in the CSP fix: script-src must never carry 'unsafe-eval' or 'unsafe-inline'
// anywhere (that would defeat the nonce), while style-src is deliberately relaxed to
// 'unsafe-inline' only under /app/ — see internal/http/middleware.go. Fetches from
// inside the already-navigated page rather than via the standalone `request` fixture:
// that fixture is a plain Node HTTP client with no route to the container network at
// all (no Chromium, so the host-resolver-rules mapping the domain to caddy doesn't
// apply to it) — found by this suite's own first run against this exact test.
test("the portal's CSP never weakens script-src, and relaxes style-src only for /app/", async ({ page }) => {
  await page.goto("/app/");
  const spaCsp = await page.evaluate(async () => {
    const res = await fetch("/app/", { cache: "no-store" });
    return res.headers.get("content-security-policy") || "";
  });
  expect(spaCsp).not.toContain("unsafe-eval");
  expect(spaCsp).toMatch(/script-src 'self' 'nonce-/);
  expect(spaCsp).toContain("style-src 'self' 'unsafe-inline'");

  const htmlCsp = await page.evaluate(async () => {
    const res = await fetch("/login", { cache: "no-store" });
    return res.headers.get("content-security-policy") || "";
  });
  expect(htmlCsp).not.toContain("unsafe-inline");
  expect(htmlCsp).not.toContain("unsafe-eval");
});
