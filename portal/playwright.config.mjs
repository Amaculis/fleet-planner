import { defineConfig, devices } from "@playwright/test";

// Runs against a real, already-deployed instance of the app, not `vite dev` or a mock:
// CSP headers, cookies and asset serving only exist in the real Go server, and
// those are what the suite is there to check.
//
// It goes through Caddy (TLS), not straight to the app container: the session and CSRF
// cookies carry the __Host- prefix, which browsers refuse to store over plain HTTP, so
// every login would fail CSRF. E2E_DOMAIN (must match APP_DOMAIN) is mapped to the
// caddy container via Chromium's host-resolver-rules (see scripts/verify-e2e.sh). That
// is a browser launch flag, so it applies to page navigations only, not to the
// standalone `request` fixture; tests that need the network from the page use
// fetch() inside it (see e2e/smoke.spec.js's CSP test).
const domain = process.env.E2E_DOMAIN;

// --ignore-certificate-errors: Caddy serves the test domain with its own local CA.
// ignoreHTTPSErrors below covers page loads, but Chromium still refuses to register a
// service worker over an untrusted certificate (SecurityError), so the PWA specs need
// the browser itself to accept it. Test browser only; production has a real certificate.
const launchOptions = {
  args: [...(domain ? [`--host-resolver-rules=MAP ${domain} caddy`] : []), "--ignore-certificate-errors"],
};

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
        launchOptions,
      },
    },
    {
      name: "chromium",
      dependencies: ["setup"],
      testIgnore: /rateLimit\.spec\.js/,
      use: {
        ...devices["Desktop Chrome"],
        launchOptions,
        // All tests but the login flow itself reuse one real login (see
        // e2e/auth.setup.js) instead of each logging in independently — avoids
        // hitting the login rate limiter under parallel execution, and is faster.
        storageState: "e2e/.auth/admin.json",
      },
    },
    {
      // Empties the login bucket on purpose, so it runs alone, after everything else.
      name: "rate-limit",
      dependencies: ["chromium"],
      testMatch: /rateLimit\.spec\.js/,
      use: {
        ...devices["Desktop Chrome"],
        launchOptions,
        storageState: "e2e/.auth/admin.json",
      },
    },
  ],
});
