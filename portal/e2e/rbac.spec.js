import { test, expect } from "@playwright/test";

// CLAUDE.md's testing minimums require role-boundary coverage: "driver cannot
// read/write another driver's or admin's data". Enforcement is server-side only
// (internal/http/server.go's RequireRole groups) — the SPA has no client-side route
// guard by design (see routes.js), so a driver hitting a planner/admin page must be
// blocked by the API responses themselves, not by the frontend hiding a link.
test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

async function createDriverSession(page) {
  const suffix = Date.now() % 100000;
  const email = `e2e-rbac-driver-${suffix}@example.com`;
  const password = "correct-horse-battery-staple";

  await page.goto("/app/");
  await page.evaluate(
    async ({ email, password }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const driverResp = await fetch("/api/drivers", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ fullName: "RBAC Test Driver", licenseExpiry: "", isActive: true }),
      });
      const driver = await driverResp.json();
      await fetch("/api/users", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ email, password, role: "driver", driverId: driver.id }),
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
}

test("driver role is rejected by planner and admin API endpoints", async ({ page }) => {
  await createDriverSession(page);

  const statuses = await page.evaluate(async () => {
    const me = await fetch("/api/auth/me").then((r) => r.json());
    const get = (url) => fetch(url).then((r) => r.status);
    const post = (url, body) =>
      fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify(body || {}),
      }).then((r) => r.status);

    return {
      // Planner-only (admin + dispatcher).
      buses: await get("/api/buses"),
      driversPlanning: await get("/api/drivers"),
      trips: await get("/api/trips"),
      clients: await get("/api/clients"),
      createClient: await post("/api/clients", { name: "Driver-made client" }),
      createTrip: await post("/api/trips", { origin: "x", destination: "y", scheduledStart: "2099-01-01T00:00", scheduledEnd: "2099-01-01T01:00" }),
      // Admin-only.
      users: await get("/api/users"),
      createBus: await post("/api/buses", { plate: "ZZ-0000", model: "x", seats: 10, status: "active" }),
      anonymizeClient: await post("/api/clients/1/anonymize"),
      createUser: await post("/api/users", { email: "nope@example.com", password: "correct-horse-battery-staple", role: "dispatcher" }),
    };
  });

  for (const [name, status] of Object.entries(statuses)) {
    expect(status, `${name} should reject a driver session`).toBe(403);
  }
});

test("driver's nav has no links to planner/admin pages, and visiting one directly yields no data", async ({ page }) => {
  await createDriverSession(page);
  await page.waitForTimeout(500);

  // MainLayout.vue gates its nav links by role (isPlanner/isAdmin) — a driver's menu
  // must not offer a way into pages whose data they can't load anyway.
  for (const label of [/^buses$/i, /^drivers$/i, /^trips$/i, /^clients$/i, /^users$/i]) {
    await expect(page.getByRole("link", { name: label })).toHaveCount(0);
  }
  await expect(page.getByRole("link", { name: /my trips/i })).toBeVisible();

  // Navigating there directly hits the client-side route guard (meta.access in
  // routes.js) and shows the 403 page without loading any data...
  const errors = [];
  page.on("pageerror", (e) => errors.push(String(e)));
  await page.goto("/app/buses");
  await expect(page).toHaveURL(/\/app\/forbidden$/);
  await expect(page.getByRole("heading", { name: "Access denied" }).first()).toBeVisible();
  expect(errors, `uncaught page errors visiting /app/buses as a driver:\n${errors.join("\n")}`).toEqual([]);

  // ...but the guard is presentation only: the real boundary is the API itself.
  const status = await page.evaluate(() => fetch("/api/buses").then((r) => r.status));
  expect(status, "GET /api/buses as a driver").toBe(403);
});
