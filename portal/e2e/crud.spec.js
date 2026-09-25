import { test, expect } from "@playwright/test";

// Exercises every add/edit/delete action in the SPA. Each test picks a unique
// identifier (Date.now()) so repeated runs never collide on a unique-field
// constraint (a bus plate, for instance) and each test cleans up what it created.
test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

test("bus: add, edit, delete", async ({ page }) => {
  const plate = `QQ-${Date.now() % 100000}`;

  await page.goto("/app/buses");
  await page.getByRole("button", { name: /add bus/i }).click();
  await expect(page).toHaveURL(/buses\/new/);

  await page.getByLabel(/plate/i).fill(plate);
  await page.getByLabel(/model/i).fill("Test Model");
  await page.getByRole("button", { name: /save/i }).click();
  await expect(page).toHaveURL(/\/app\/buses$/, { timeout: 10000 });
  await expect(page.getByText(plate)).toBeVisible();

  // Edit: LxDataGrid renders per-row actions as direct icon buttons, not a menu.
  const row = page.getByRole("row", { name: new RegExp(plate) });
  await row.getByRole("button", { name: /edit/i }).click();
  await expect(page).toHaveURL(/buses\/\d+\/edit/);
  await page.getByLabel(/model/i).fill("Edited Model");
  await page.getByRole("button", { name: /save/i }).click();
  await expect(page).toHaveURL(/\/app\/buses$/, { timeout: 10000 });
  await expect(page.getByText("Edited Model")).toBeVisible();

  // Delete: confirm dialog defaults to Yes/No (LxShell's own confirmModal*DefaultLabel
  // texts — see src/hooks/shellTexts.js) since pushSimple() doesn't set custom labels.
  const editedRow = page.getByRole("row", { name: new RegExp(plate) });
  await editedRow.getByRole("button", { name: /delete/i }).click();
  await page.getByRole("button", { name: /^yes$/i }).click();
  await expect(page.getByText(plate)).toHaveCount(0);
});

test("driver: add, edit, anonymize", async ({ page }) => {
  const name = `Test Driver ${Date.now() % 100000}`;

  await page.goto("/app/drivers");
  await page.getByRole("button", { name: /add driver/i }).click();
  await expect(page).toHaveURL(/drivers\/new/);
  await page.getByLabel(/full name/i).fill(name);
  await page.getByRole("button", { name: /save/i }).click();
  await expect(page).toHaveURL(/\/app\/drivers$/, { timeout: 10000 });
  // LxDataGrid virtualizes rows (hasVirtualization defaults to true) — with 50+
  // drivers from earlier test runs, a freshly created row's DOM node genuinely
  // doesn't exist yet outside the rendered window, not just off-screen. Search
  // narrows the grid down to the one match instead of scrolling to find it.
  await page.getByRole("textbox", { name: /search/i }).fill(name);
  await expect(page.getByText(name)).toBeVisible();

  const row = page.getByRole("row", { name: new RegExp(name) });
  await row.getByRole("button", { name: /edit/i }).click();
  await expect(page).toHaveURL(/drivers\/\d+\/edit/);
  await page.getByLabel(/phone/i).fill("20000000");
  await page.getByRole("button", { name: /save/i }).click();
  await expect(page).toHaveURL(/\/app\/drivers$/, { timeout: 10000 });

  // The edit round trip remounts the list (searchTerm is component-local state),
  // so the search box is empty again — same virtualization reasoning as above.
  await page.getByRole("textbox", { name: /search/i }).fill(name);

  // Anonymize (GDPR erasure) — the driver's name is replaced, so it must stop
  // appearing under its original name.
  const editedRow = page.getByRole("row", { name: new RegExp(name) });
  await editedRow.getByRole("button", { name: /erase/i }).click();
  await page.getByRole("button", { name: /^yes$/i }).click();
  await expect(page.getByText(name)).toHaveCount(0);
});

test("trip: add, view detail, edit, delete", async ({ page }) => {
  const origin = `TestOrigin${Date.now() % 100000}`;

  await page.goto("/app/trips");
  await page.getByRole("button", { name: /add trip/i }).click();
  await expect(page).toHaveURL(/trips\/new/);

  await page.getByLabel(/^origin/i).fill(origin);
  await page.getByLabel(/^destination/i).fill("TestDestination");

  const start = new Date(Date.now() + 24 * 60 * 60 * 1000);
  const end = new Date(start.getTime() + 4 * 60 * 60 * 1000);
  // The picker's own displayed/expected typed format is "DD.MM.YYYY. HH:MM" (visible
  // in its own placeholder and in what it echoes back after a successful entry) —
  // confirmed by screenshot, not assumed; typing an ISO-shaped string instead let the
  // mask silently misparse into a nonsense date (e.g. a different year entirely),
  // which is what made this test intermittently fail with a real but wrong
  // validation error ("longer than 30 days") rather than a crash.
  const fmt = (d) =>
    `${String(d.getDate()).padStart(2, "0")}.${String(d.getMonth() + 1).padStart(2, "0")}.${d.getFullYear()}. ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  // LxDateTimePicker parses its typed text into a Date on blur, not on every
  // keystroke — Tab away from each field before saving, or the v-model this app's
  // DateTimeField.vue wrapper reads from never updates and the form submits empty.
  await page.getByLabel(/scheduled start/i).fill(fmt(start));
  await page.keyboard.press("Tab");
  await page.getByLabel(/scheduled end/i).fill(fmt(end));
  await page.keyboard.press("Tab");
  await page.getByRole("button", { name: /save/i }).click();

  // Lands on the trip detail page after a successful create.
  await expect(page).toHaveURL(/\/app\/trips\/\d+$/, { timeout: 10000 });
  await expect(page.getByText(new RegExp(origin))).toBeVisible();

  // Edit from the detail page.
  await page.getByRole("button", { name: /^edit$/i }).click();
  await expect(page).toHaveURL(/trips\/\d+\/edit/);
  await page.getByLabel(/notes/i).fill("Edited via e2e");
  await page.getByRole("button", { name: /save/i }).click();
  await expect(page).toHaveURL(/\/app\/trips\/\d+$/, { timeout: 10000 });
  await expect(page.getByText("Edited via e2e")).toBeVisible();

  // Delete from the detail page.
  await page.getByRole("button", { name: /^delete$/i }).click();
  await page.getByRole("button", { name: /^yes$/i }).click();
  await expect(page).toHaveURL(/\/app\/trips$/, { timeout: 10000 });
  await expect(page.getByText(new RegExp(origin))).toHaveCount(0);
});

test("user: add, activate/deactivate", async ({ page }) => {
  const email = `test-${Date.now() % 100000}@example.com`;

  await page.goto("/app/users");
  await page.getByRole("button", { name: /add user/i }).click();
  await expect(page).toHaveURL(/users\/new/);

  await page.getByLabel(/^email/i).fill(email);
  await page.locator('input[type="password"]').fill("correct-horse-battery-staple");
  await page.getByRole("button", { name: /save/i }).click();
  await expect(page).toHaveURL(/\/app\/users$/, { timeout: 10000 });
  // Same reasoning as the driver test above: LxDataGrid virtualizes rows, so a
  // freshly created user's row may not exist in the DOM yet without searching.
  await page.getByRole("textbox", { name: /search/i }).fill(email);
  await expect(page.getByText(email)).toBeVisible();

  // Deactivate, then reactivate — the one row action users currently have.
  const row = page.getByRole("row", { name: new RegExp(email) });
  await row.getByRole("button").click();
  await expect(page.getByRole("row", { name: new RegExp(email) }).getByText(/^no$/i)).toBeVisible();
  await page.getByRole("row", { name: new RegExp(email) }).getByRole("button").click();
  await expect(page.getByRole("row", { name: new RegExp(email) }).getByText(/^yes$/i)).toBeVisible();
});
