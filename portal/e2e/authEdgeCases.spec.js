import { test, expect } from "@playwright/test";
import { login, ADMIN_EMAIL, ADMIN_PASSWORD } from "./support.js";

// The remaining CLAUDE.md auth minimums smoke.spec.js doesn't cover: logout, session
// expiry, CSRF rejection, and the login rate limiter actually triggering.
test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

// Every other test in this suite reuses one shared admin session (playwright.config.mjs's
// storageState) — actually logging it out here would invalidate that same session
// server-side for every other test/file still to run against this container. This test
// logs in for itself instead (same pattern as smoke.spec.js's own login test), on its own
// disposable session, so it can safely kill only that one.
test.describe("logout", () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  test("logging out invalidates the session server-side, not just client-side", async ({ page }) => {
    await login(page, ADMIN_EMAIL, ADMIN_PASSWORD);

    await page.getByRole("button", { name: "User menu" }).first().click();
    await page.getByRole("button", { name: /^log out$/i }).click();
    await page.waitForURL(/\/app\/login$/);

    // The real assertion: the old session is dead on the server, not just forgotten by
    // the SPA's in-memory state. A stale client (e.g. a second tab) hitting a protected
    // endpoint after logout must be treated as anonymous.
    const status = await page.evaluate(() => fetch("/api/auth/me").then((r) => r.status));
    expect(status).toBe(200); // /api/auth/me always answers 200, authenticated:false when signed out
    const authed = await page.evaluate(() => fetch("/api/auth/me").then((r) => r.json()));
    expect(authed.authenticated).toBe(false);

    const tripsStatus = await page.evaluate(() => fetch("/api/trips").then((r) => r.status));
    expect(tripsStatus).toBe(401);
  });
});

// Waiting out the real 4h idle / 12h absolute timeout (internal/config/config.go) isn't
// practical in e2e. A tampered/garbage session cookie produces the same observable
// behavior an expired one would (the server no longer recognizes the opaque token) —
// this is the closest e2e-reachable proxy for "the session is no longer valid".
test("a session with an invalid/expired cookie is treated as signed out, not as a crash", async ({ page }) => {
  await page.goto("/app/");
  await page.waitForTimeout(500);

  const cookies = await page.context().cookies();
  const sessionCookie = cookies.find((c) => /session/i.test(c.name));
  expect(sessionCookie, "a session cookie must exist once logged in").toBeTruthy();

  await page.context().addCookies([{ ...sessionCookie, value: `${sessionCookie.value}-tampered` }]);

  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto("/app/trips");
  // The redirect carries the original path (?redirect=/trips) so login can return the
  // user there afterwards — not asserted here, just not excluded by the URL match.
  await page.waitForURL(/\/app\/login/, { timeout: 10000 });
  expect(errors, `uncaught page errors after an invalidated session:\n${errors.join("\n")}`).toEqual([]);
});

test("a state-changing request without a valid CSRF token is rejected", async ({ page }) => {
  await page.goto("/app/");
  await page.waitForTimeout(500);

  const results = await page.evaluate(async () => {
    const noToken = await fetch("/api/buses", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ plate: "CS-0001", model: "x", seats: 10, status: "active" }),
    });
    const wrongToken = await fetch("/api/buses", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": "not-a-real-token" },
      body: JSON.stringify({ plate: "CS-0002", model: "x", seats: 10, status: "active" }),
    });
    return { noToken: noToken.status, wrongToken: wrongToken.status };
  });

  expect(results.noToken, "POST with no X-CSRF-Token header").toBe(403);
  expect(results.wrongToken, "POST with a garbage X-CSRF-Token").toBe(403);
});

// Runs last and deliberately exhausts the shared per-IP login bucket (5/minute, see
// internal/http/server.go's loginLimiter) — any test after this one in the same
// container run would itself start failing logins, hence it living alone at the end
// of its own file and the container needing a restart before other login-heavy specs
// run again (documented in the verify-e2e workflow, not something this test can fix).
test("the login rate limiter trips after repeated attempts from the same client", async ({ page }) => {
  await page.goto("/app/");

  const statuses = await page.evaluate(async () => {
    const me = await fetch("/api/auth/me").then((r) => r.json());
    const results = [];
    for (let i = 0; i < 10; i++) {
      const resp = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ email: "nobody@example.com", password: "wrong-password" }),
      });
      results.push(resp.status);
      if (resp.status === 429) break;
    }
    return results;
  });

  expect(statuses, "login attempts should eventually hit 429").toContain(429);
});
