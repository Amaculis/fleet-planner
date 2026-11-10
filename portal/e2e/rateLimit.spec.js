import { test, expect } from "@playwright/test";

// The per-IP login limit (internal/http/server.go's loginLimiter). Sends three times
// the burst at once, so the bucket must run dry however fast it refills. Every attempt
// uses a different email, so the per-account limit can't be what answers 429.
// E2E_LOGIN_BURST matches docker-compose.e2e.yml (scripts/verify-e2e.sh sets it); the
// fallback is the production default.
//
// Its own file and its own Playwright project ("rate-limit" in playwright.config.mjs),
// run only after every other spec has finished: while the bucket is empty, any other
// spec's login would get a 429 and hang.
test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

const LOGIN_BURST = Number(process.env.E2E_LOGIN_BURST || 5);

test("the login rate limiter trips after repeated attempts from the same client", async ({ page }) => {
  await page.goto("/app/");

  const statuses = await page.evaluate(async (attempts) => {
    const me = await fetch("/api/auth/me").then((r) => r.json());
    const run = Date.now();
    const requests = Array.from({ length: attempts }, (_, i) =>
      fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ email: `nobody-${run}-${i}@example.com`, password: "wrong-password" }),
      }).then((r) => r.status)
    );
    return Promise.all(requests);
  }, LOGIN_BURST * 3);

  expect(statuses, "login attempts should eventually hit 429").toContain(429);
});
