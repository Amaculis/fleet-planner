import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

// The portal as an installable app (PWA): manifest, service worker (src/pwa/sw.js),
// offline start, and — the part that matters most — what the worker must never keep.
test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

// Opens the portal and waits until the service worker controls the page. The first
// visit only installs it; clients.claim() in its activate step takes the page over.
async function openControlled(page, path = "/app/") {
  await page.goto(path);
  await page.evaluate(() => navigator.serviceWorker.ready);
  await expect
    .poll(() => page.evaluate(() => !!navigator.serviceWorker.controller), { timeout: 10000 })
    .toBe(true);
}

test("the manifest is linked and describes an installable app under /app/", async ({ page }) => {
  await page.goto("/app/");
  const href = await page.locator('link[rel="manifest"]').getAttribute("href");
  expect(href).toBe("/app/manifest.webmanifest");

  const result = await page.evaluate(async (url) => {
    const res = await fetch(url, { cache: "no-store" });
    const manifest = await res.json();
    // Paths in the manifest are relative to it (the demo build lives under another base).
    const abs = (path) => new URL(path, res.url).pathname;
    for (const key of ["id", "start_url", "scope"]) manifest[key] = abs(manifest[key]);
    const icons = await Promise.all(
      manifest.icons.map(async (icon) => {
        icon.src = abs(icon.src);
        const r = await fetch(icon.src);
        const bytes = new Uint8Array(await r.arrayBuffer());
        return { src: icon.src, status: r.status, png: bytes[0] === 0x89 && bytes[1] === 0x50 };
      })
    );
    return { contentType: res.headers.get("content-type"), manifest, icons };
  }, href);

  expect(result.contentType).toContain("application/manifest+json");
  expect(result.manifest).toMatchObject({ start_url: "/app/", scope: "/app/", id: "/app/", display: "standalone" });
  expect(result.manifest.icons.some((i) => i.sizes === "192x192")).toBe(true);
  expect(result.manifest.icons.some((i) => i.sizes === "512x512" && i.purpose === "maskable")).toBe(true);
  for (const icon of result.icons) {
    expect(icon.status, icon.src).toBe(200);
    expect(icon.png, `${icon.src} is a PNG`).toBe(true);
  }
});

test("Chrome considers the app installable", async ({ page }) => {
  await openControlled(page);
  const cdp = await page.context().newCDPSession(page);
  const { installabilityErrors } = await cdp.send("Page.getInstallabilityErrors");
  expect(installabilityErrors).toEqual([]);
});

test("the service worker registers for /app/ without console errors", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  await openControlled(page);

  const info = await page.evaluate(async () => {
    const reg = await navigator.serviceWorker.getRegistration("/app/");
    return { scope: reg?.scope, script: reg?.active?.scriptURL };
  });
  expect(new URL(info.scope).pathname).toBe("/app/");
  expect(new URL(info.script).pathname).toBe("/app/pwa-sw.js");
  expect(errors, `console errors:\n${errors.join("\n")}`).toEqual([]);
});

test("the worker's cache is named after the deployed build", async ({ page }) => {
  await openControlled(page);
  const { version, keys } = await page.evaluate(async () => ({
    version: (await fetch("/app/version.json", { cache: "no-store" }).then((r) => r.json())).version,
    keys: await caches.keys(),
  }));
  expect(keys).toContain(`fleet-portal-${version}`);
});

test("nothing personal is cached: no API responses, only the app's own files", async ({ page }) => {
  await openControlled(page);
  // Visit pages that load plenty of personal data through the API.
  for (const path of ["/app/trips", "/app/drivers", "/app/users", "/app/"]) {
    await page.goto(path);
    await page.waitForLoadState("networkidle");
  }

  const cached = await page.evaluate(async () => {
    const urls = [];
    for (const key of await caches.keys()) {
      const cache = await caches.open(key);
      for (const req of await cache.keys()) urls.push(new URL(req.url).pathname);
    }
    return urls;
  });

  expect(cached.length, "the app shell and its files are cached").toBeGreaterThan(0);
  for (const path of cached) {
    expect(path.startsWith("/app/"), `${path} is outside the app`).toBe(true);
    expect(path, "API responses must never be cached").not.toContain("/api/");
    expect(path, "the update check must always reach the network").not.toBe("/app/version.json");
  }
});

// Loads the given pages once online, so their code is in the worker's cache.
async function warmUp(page, paths) {
  for (const path of paths) {
    await page.goto(path);
    await page.waitForLoadState("networkidle");
  }
}

test("started offline, the app shows its offline page and recovers by itself", async ({ page, context }) => {
  await openControlled(page);
  await warmUp(page, ["/app/trips", "/app/"]);

  await context.setOffline(true);
  try {
    await page.goto("/app/trips");
    // The worker answers with offline.html, but the address stays the user's page.
    await expect(page).toHaveURL(/\/app\/trips$/);
    await expect(page.locator("#offline-title")).toHaveText("No connection");
    await expect(page.locator("#offline-title")).toHaveCSS("color", /rgb/);
    await expect(page.getByRole("button", { name: /try again/i })).toBeVisible();
    // Its styles come from the cache too: the lx-ui font family is applied.
    await expect(page.locator("main")).toHaveCSS("font-family", /IBM Plex Sans/i);
  } finally {
    await context.setOffline(false);
  }

  // Back online: the page notices within seconds, reloads, and the real page is back.
  await expect(page.getByText(/^trips$/i).first()).toBeVisible({ timeout: 15000 });
  await expect(page.locator("#offline-title")).toHaveCount(0);
});

test("the offline page speaks the language the user picked", async ({ page, context }) => {
  await openControlled(page);
  await page.evaluate(() => localStorage.setItem("bus-fleet-locale", "ru"));
  await context.setOffline(true);
  try {
    await page.goto("/app/");
    await expect(page.locator("#offline-title")).toHaveText("Нет связи");
    await expect(page.locator("html")).toHaveAttribute("lang", "ru");
  } finally {
    // Each test has a fresh browser context, so the saved language goes with it.
    await context.setOffline(false);
  }
});

test("if the server doesn't answer at start, the app says so and retries", async ({ page }) => {
  await page.route("**/api/auth/me", (route) => route.abort("connectionrefused"));
  await page.goto("/app/trips");
  await expect(page).toHaveURL(/\/app\/offline\?redirect=/);
  await expect(page.getByText("No connection", { exact: true }).first()).toBeVisible();

  // Server is back: "Try again" goes on to the page the user wanted.
  await page.unroute("**/api/auth/me");
  await page.getByRole("button", { name: /try again/i }).click();
  await expect(page).toHaveURL(/\/app\/trips$/);
});

test("losing the connection inside the app says so and keeps the user where they are", async ({ page, context }) => {
  await openControlled(page);
  await warmUp(page, ["/app/trips", "/app/"]);
  // The dashboard must be fully loaded first (its data and icons), or going offline
  // mid-load is what the test ends up observing.
  await expect(page.getByText(/active buses/i).first()).toBeVisible();
  await page.waitForLoadState("networkidle");

  await context.setOffline(true);
  try {
    // lx-ui (2.3.12+) cancels in-app navigation while offline and shows its own
    // notice, in the app's language (lxComponentTexts in i18n/index.js).
    await page.getByRole("link", { name: /^trips$/i }).first().click();
    await expect(page.getByText(/no internet connection/i).first()).toBeVisible({ timeout: 10000 });
    await expect(page).toHaveURL(/\/app\/$/);
  } finally {
    await context.setOffline(false);
  }

  // Back online, the same click works.
  await page.getByRole("link", { name: /^trips$/i }).first().click();
  await expect(page).toHaveURL(/\/app\/trips/);
});

test("offline, starting or finishing a trip is not faked: the request fails", async ({ page, context }) => {
  await openControlled(page);
  await context.setOffline(true);
  try {
    const outcome = await page.evaluate(async () => {
      try {
        const r = await fetch("/api/my/trips/1/start", { method: "POST" });
        return `status ${r.status}`;
      } catch {
        return "network error";
      }
    });
    expect(outcome).toBe("network error");
  } finally {
    await context.setOffline(false);
  }
});

// The server-rendered UI has its own worker (/sw.js, scope "/"). A phone that used it
// before must still end up with the portal's worker in charge of /app/.
test("the server-rendered UI's worker doesn't take over the portal", async ({ page }) => {
  await page.goto("/login");
  await page.evaluate(() => navigator.serviceWorker.ready);
  const rootScope = await page.evaluate(async () => (await navigator.serviceWorker.getRegistration("/"))?.scope);
  expect(new URL(rootScope).pathname).toBe("/");

  await openControlled(page);
  await page.reload();
  const controller = await page.evaluate(() => navigator.serviceWorker.controller?.scriptURL);
  expect(new URL(controller).pathname).toBe("/app/pwa-sw.js");
});
