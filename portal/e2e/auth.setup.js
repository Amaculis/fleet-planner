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
  await seedFleet(page);
});

// Several specs book "any assignable bus/driver" (dashboard, myTrips, paymentStatus,
// tripDetail). On a fresh database (CI) there are none until some other spec happens
// to create one, so make sure a few exist before any spec runs. Idempotent: the fixed
// plates and names are only created when missing.
const SEED_BUSES = 8;
const SEED_DRIVERS = 4;

async function seedFleet(page) {
  const failures = await page.evaluate(
    async ({ buses, drivers }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const post = (url, body) =>
        fetch(url, {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
          body: JSON.stringify(body),
        });
      const failed = [];

      const plates = new Set((await fetch("/api/buses").then((r) => r.json())).map((b) => b.plate));
      for (let i = 1; i <= buses; i++) {
        const plate = `E2E-SEED-${i}`;
        if (plates.has(plate)) continue;
        const r = await post("/api/buses", { plate, model: "E2E Seed", seats: 50, status: "active" });
        if (!r.ok) failed.push(`bus ${plate}: ${r.status}`);
      }

      const names = new Set((await fetch("/api/drivers").then((r) => r.json())).map((d) => d.fullName));
      for (let i = 1; i <= drivers; i++) {
        const fullName = `E2E Seed Driver ${i}`;
        if (names.has(fullName)) continue;
        const r = await post("/api/drivers", { fullName, licenseExpiry: "", isActive: true });
        if (!r.ok) failed.push(`driver ${fullName}: ${r.status}`);
      }
      return failed;
    },
    { buses: SEED_BUSES, drivers: SEED_DRIVERS }
  );
  if (failures.length) throw new Error(`seeding the fleet failed: ${failures.join(", ")}`);
}
