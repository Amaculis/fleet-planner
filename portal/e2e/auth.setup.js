import { test as setup } from "@playwright/test";
import { login, ADMIN_EMAIL, ADMIN_PASSWORD } from "./support.js";

const STORAGE_STATE = "e2e/.auth/admin.json";

setup.skip(!ADMIN_EMAIL || !ADMIN_PASSWORD, "E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD not set");

// Logs in once; every other test reuses this via projects[].use.storageState (see
// playwright.config.mjs) instead of calling login() itself. Found the hard way: with
// every test logging in independently under full parallelism, 8 simultaneous logins
// from this one container's single IP blew straight through the login rate limiter
// (5 per minute — internal/http/server.go's loginLimiter), so some logins hung
// waiting for a redirect that a 429 was never going to send. One real login covers
// what all of them actually need to test.
setup("authenticate as admin", async ({ page }) => {
  await login(page, ADMIN_EMAIL, ADMIN_PASSWORD);
  await page.context().storageState({ path: STORAGE_STATE });
});
