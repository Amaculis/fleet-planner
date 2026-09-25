import { defineConfig, devices } from "@playwright/test";

// Runs against a real, already-deployed instance of the app — not against `vite dev`
// or a mocked server. The whole reason for this suite is that this session's actual
// bugs (missing CSS modules, a CSP eval violation, invalid icon names, a CSP directive
// silently ignored by one engine) were only ever visible in a real browser hitting the
// real Go server, which sets the CSP header itself (see internal/http/middleware.go) —
// none of that exists under jsdom or a dev server with different asset serving.
//
// This goes through Caddy (TLS), not straight to the app container over plain HTTP —
// found the hard way, via this suite's own first real run: the session cookie and the
// CSRF cookie both carry the __Host- prefix, which browsers refuse to persist over an
// insecure origin (Secure is a __Host- requirement). Hitting the app directly over
// HTTP meant every request got a cookie the browser silently dropped, so the CSRF
// token from one response could never match the nonce on the next request — a 403 on
// every login, not a bug in the app, an artifact of skipping Caddy. E2E_DOMAIN (must
// match APP_DOMAIN, so Caddy's site block and TLS SNI resolve) is mapped to the caddy
// container by IP via Chromium's host-resolver-rules — see scripts/verify-e2e.sh —
// rather than touching a hosts file. That mapping is a Chromium launch flag, so it
// only applies to real page navigations, never to the standalone `request` fixture
// (a plain Node HTTP client with no route to the container network at all) — see
// e2e/smoke.spec.js's CSP test, which fetches from inside the page instead for
// exactly this reason.
const domain = process.env.E2E_DOMAIN;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [["list"]],
  use: {
    baseURL: domain ? `https://${domain}` : "http://localhost:8080",
    ignoreHTTPSErrors: true,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "setup",
      testMatch: /auth\.setup\.js/,
      use: {
        launchOptions: domain
          ? { args: [`--host-resolver-rules=MAP ${domain} caddy`] }
          : {},
      },
    },
    {
      name: "chromium",
      dependencies: ["setup"],
      use: {
        ...devices["Desktop Chrome"],
        launchOptions: domain
          ? { args: [`--host-resolver-rules=MAP ${domain} caddy`] }
          : {},
        // All tests but the login flow itself reuse one real login (see
        // e2e/auth.setup.js) instead of each logging in independently — avoids
        // hitting the login rate limiter under parallel execution, and is faster.
        storageState: "e2e/.auth/admin.json",
      },
    },
  ],
});
