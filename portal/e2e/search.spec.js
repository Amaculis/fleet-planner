import { test, expect } from "@playwright/test";

// crud.spec.js uses the driver/user search boxes incidentally (to find a freshly
// created row past LxDataGrid's virtualization window), but never checks that
// filtering itself actually narrows or widens results. This exercises the search
// feature directly: a narrow query matches only one row, a shared-substring query
// matches both. (Clearing the search back to "" isn't asserted here — with 50+ rows
// accumulated from other e2e runs, an unfiltered grid virtualizes these two rows
// straight back out of the DOM, the same root cause crud.spec.js's comments
// document; that's a virtualization fact, not something this search feature could
// ever make visible without a query narrowing the grid.)
test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

test("driver list search filters by name, and a shared substring matches both", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const shared = `SearchDrv${suffix}`;
  const nameA = `${shared}Alpha`;
  const nameB = `${shared}Bravo`;

  await page.goto("/app/");
  await page.evaluate(
    async ({ nameA, nameB }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      for (const fullName of [nameA, nameB]) {
        await fetch("/api/drivers", {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
          body: JSON.stringify({ fullName, licenseExpiry: "", isActive: true }),
        });
      }
    },
    { nameA, nameB }
  );

  await page.goto("/app/drivers");
  const search = page.getByRole("textbox", { name: /search/i });

  await search.fill(nameA);
  await expect(page.getByText(nameA)).toBeVisible();
  await expect(page.getByText(nameB)).toHaveCount(0);

  await search.fill(shared);
  await expect(page.getByText(nameA)).toBeVisible();
  await expect(page.getByText(nameB)).toBeVisible();
});

test("user list search filters by email, and a shared substring matches both", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const shared = `search-usr-${suffix}`;
  const emailA = `${shared}-charlie@example.com`;
  const emailB = `${shared}-delta@example.com`;

  await page.goto("/app/");
  await page.evaluate(
    async ({ emailA, emailB }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      for (const email of [emailA, emailB]) {
        await fetch("/api/users", {
          method: "POST",
          headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
          body: JSON.stringify({ email, password: "correct-horse-battery-staple", role: "dispatcher" }),
        });
      }
    },
    { emailA, emailB }
  );

  await page.goto("/app/users");
  const search = page.getByRole("textbox", { name: /search/i });

  await search.fill(emailA);
  await expect(page.getByText(emailA)).toBeVisible();
  await expect(page.getByText(emailB)).toHaveCount(0);

  await search.fill(shared);
  await expect(page.getByText(emailA)).toBeVisible();
  await expect(page.getByText(emailB)).toBeVisible();
});
