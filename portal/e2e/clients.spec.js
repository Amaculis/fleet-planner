import { test, expect } from "@playwright/test";
import { trackConsoleErrors, gridSearch } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

async function apiPost(page, path, body) {
  return page.evaluate(
    async ({ path, body }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const resp = await fetch(`/api${path}`, {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify(body),
      });
      return { status: resp.status, body: await resp.json() };
    },
    { path, body }
  );
}

function tripBody(suffix, extra = {}) {
  const pad = (n) => String(n).padStart(2, "0");
  const f = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  const start = new Date(Date.now() + 40 * 24 * 60 * 60 * 1000);
  start.setHours(10, 0, 0, 0);
  const end = new Date(start.getTime() + 2 * 60 * 60 * 1000);
  return {
    origin: `ClientOrigin${suffix}`,
    destination: `ClientDest${suffix}`,
    scheduledStart: f(start),
    scheduledEnd: f(end),
    paymentStatus: "unpaid",
    notes: null,
    ...extra,
  };
}

test("a planner can create a client from the Clients page, with no console errors", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  const suffix = Date.now() % 100000;

  await page.goto("/app/clients");
  await page.waitForTimeout(500);
  await page.getByRole("button", { name: /add client/i }).click();
  await page.waitForURL(/\/app\/clients\/new$/);

  await page.getByLabel(/^client name/i).fill(`E2E Client ${suffix}`);
  await page.getByLabel(/^contact person/i).fill("Test Contact");
  await page.getByLabel(/^phone/i).fill("+371 20000000");
  await page.getByLabel(/^email/i).fill(`client${suffix}@example.test`);

  const [resp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/api/clients") && r.request().method() === "POST"),
    page.getByRole("button", { name: /^save/i }).click(),
  ]);
  expect(resp.status()).toBe(201);
  const created = await resp.json();
  expect(created.name).toBe(`E2E Client ${suffix}`);

  await expect(page).toHaveURL(/\/app\/clients$/);
  await gridSearch(page).fill(`E2E Client ${suffix}`);
  await expect(page.getByText(`E2E Client ${suffix}`).first()).toBeVisible();

  expect(errors, `console errors creating a client:\n${errors.join("\n")}`).toEqual([]);
});

test("client form rejects a missing name and a malformed email before sending", async ({ page }) => {
  await page.goto("/app/clients/new");
  await page.waitForTimeout(500);
  await page.getByLabel(/^email/i).fill("not-an-email");
  await page.getByRole("button", { name: /^save/i }).click();
  await expect(page.getByText("This field is required.")).toBeVisible();
  await expect(page.getByText("Enter a valid email address.")).toBeVisible();
  await expect(page).toHaveURL(/\/app\/clients\/new$/);
});

test("trip view reveals client info inline on demand and shows the bill number", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/");
  const suffix = Date.now() % 100000;

  const client = await apiPost(page, "/clients", {
    name: `Reveal Client ${suffix}`,
    contactPerson: "Reveal Contact",
    phone: "+371 20000001",
    email: `reveal${suffix}@example.test`,
    address: `1 Example Street ${suffix}`,
    notes: null,
    isActive: true,
  });
  expect(client.status).toBe(201);
  const trip = await apiPost(page, "/trips", tripBody(suffix, { clientId: client.body.id, invoiceNumber: `BILL-${suffix}` }));
  expect(trip.status).toBe(201);

  await page.goto(`/app/trips/${trip.body.id}`);
  await page.waitForTimeout(500);

  // Nothing about the client is on screen, nor fetched, until asked for.
  await expect(page.getByTestId("trip-client-info")).toHaveCount(0);
  await expect(page.getByTestId("trip-invoice")).toHaveText(`BILL-${suffix}`);

  await page.getByRole("button", { name: "Show client info" }).click();
  const info = page.getByTestId("trip-client-info");
  await expect(info).toBeVisible();
  await expect(info).toContainText(`Reveal Client ${suffix}`);
  await expect(info).toContainText("Reveal Contact");
  await expect(info).toContainText("+371 20000001");
  await expect(info).toContainText(`reveal${suffix}@example.test`);
  await expect(info).toContainText(`1 Example Street ${suffix}`);

  await page.getByRole("button", { name: "Hide client info" }).click();
  await expect(info).toHaveCount(0);

  expect(errors, `console errors on trip client reveal:\n${errors.join("\n")}`).toEqual([]);
});

test("a trip without a client says so and has no reveal button", async ({ page }) => {
  await page.goto("/app/");
  const suffix = Date.now() % 100000;
  const trip = await apiPost(page, "/trips", tripBody(suffix));
  expect(trip.status).toBe(201);

  await page.goto(`/app/trips/${trip.body.id}`);
  await page.waitForTimeout(500);
  await expect(page.getByText("No client.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Show client info" })).toHaveCount(0);
});

test("the trip form saves the chosen client and bill number", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await page.goto("/app/");
  const suffix = Date.now() % 100000;

  const client = await apiPost(page, "/clients", { name: `Form Client ${suffix}`, isActive: true });
  expect(client.status).toBe(201);
  const trip = await apiPost(page, "/trips", tripBody(suffix));
  expect(trip.status).toBe(201);

  await page.goto(`/app/trips/${trip.body.id}/edit`);
  await page.waitForTimeout(500);

  await page.getByRole("combobox", { name: "Client" }).click();
  await page.getByRole("option", { name: `Form Client ${suffix}` }).click();
  await page.getByLabel(/^bill number/i).fill(`FORM-${suffix}`);

  const [resp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes(`/api/trips/${trip.body.id}`) && r.request().method() === "PUT"),
    page.getByRole("button", { name: /^save/i }).click(),
  ]);
  expect(resp.status()).toBe(200);
  const saved = await resp.json();
  expect(saved.clientId).toBe(client.body.id);
  expect(saved.invoiceNumber).toBe(`FORM-${suffix}`);

  await expect(page.getByTestId("trip-invoice")).toHaveText(`FORM-${suffix}`);
  expect(errors, `console errors saving client on a trip:\n${errors.join("\n")}`).toEqual([]);
});

test("the API refuses a bill number over 50 characters and an unknown client", async ({ page }) => {
  await page.goto("/app/");
  const suffix = Date.now() % 100000;
  const tooLong = await apiPost(page, "/trips", tripBody(suffix, { invoiceNumber: "x".repeat(51) }));
  expect(tooLong.status).toBe(400);
  const unknown = await apiPost(page, "/trips", tripBody(suffix, { clientId: 999999999 }));
  expect([400, 404]).toContain(unknown.status);
});

// Erasing a client's personal data is admin-only. The server enforces it (see the Go
// integration tests); this checks the list doesn't offer the action to a dispatcher.
test("only an admin sees the Erase data action in the client list", async ({ page }) => {
  await page.goto("/app/");
  const suffix = Date.now() % 100000;
  const name = `EraseVis${suffix}`;
  const email = `e2e-clients-dispatcher-${suffix}@example.com`;
  const password = "correct-horse-battery-staple";

  expect((await apiPost(page, "/clients", { name, isActive: true })).status).toBe(201);
  expect((await apiPost(page, "/users", { email, password, role: "dispatcher" })).status).toBe(201);

  await page.goto("/app/clients");
  await gridSearch(page).fill(name);
  await expect(page.getByText(name).first()).toBeVisible();
  await expect(page.getByRole("button", { name: "Erase data" }).first()).toBeVisible();

  await page.context().clearCookies();
  await page.goto("/app/login");
  await page.getByRole("textbox", { name: /email/i }).fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(/\/app\/(dashboard)?$/);

  await expect(page.getByRole("link", { name: /^clients$/i })).toBeVisible();
  await page.goto("/app/clients");
  await gridSearch(page).fill(name);
  await expect(page.getByText(name).first()).toBeVisible();
  await expect(page.getByRole("button", { name: "Erase data" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Edit" }).first()).toBeVisible();
});
