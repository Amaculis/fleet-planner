import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

// The bus, driver and user forms share the trip form's structure (LxForm with sections,
// footer Save/Cancel, inline validation) — tripForm.spec.js covers the trip specifics.
test("the bus, driver and user forms all render Save and Cancel, in English, with no console errors", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  for (const path of ["/app/buses/new", "/app/drivers/new", "/app/users/new"]) {
    await page.goto(path);
    await page.waitForTimeout(500);
    await expect(page.getByRole("button", { name: /^save/i }), `${path}: Save`).toBeVisible();
    // LxForm silently drops footer actions of an unrecognised kind, and ships Latvian
    // defaults for its required markers — both regressions this guards.
    await expect(page.getByRole("button", { name: /^cancel/i }), `${path}: Cancel`).toBeVisible();
    await expect(page.getByText(/obligāts|neobligāts|Citas darbības/), `${path}: Latvian text`).toHaveCount(0);
  }
  expect(errors, `console errors on the edit forms:\n${errors.join("\n")}`).toEqual([]);
});

test("bus form: an invalid plate and a missing model are flagged inline; Cancel returns to the list", async ({ page }) => {
  await page.goto("/app/buses/new");
  await page.waitForTimeout(500);
  await page.getByLabel(/^plate/i).fill("A");
  await page.getByRole("button", { name: /^save/i }).click();

  await expect(page.getByText("Use 2–16 letters, digits or hyphens.")).toBeVisible();
  await expect(page.getByText("This field is required.")).toHaveCount(1); // the model
  await expect(page).toHaveURL(/\/app\/buses\/new$/);

  await page.getByRole("button", { name: /^cancel/i }).click();
  await expect(page).toHaveURL(/\/app\/buses$/);
});

test("driver form: a missing name, a bad phone and a bad hourly rate are each flagged inline", async ({ page }) => {
  await page.goto("/app/drivers/new");
  await page.waitForTimeout(500);
  await page.getByLabel(/phone/i).fill("abc");
  await page.getByLabel(/hourly rate/i).fill("1.234");
  await page.getByRole("button", { name: /^save/i }).click();

  await expect(page.getByText("This field is required.")).toHaveCount(1); // the name
  await expect(page.getByText(/Enter a valid phone number/)).toBeVisible();
  await expect(page.getByText(/up to 2 decimals, e\.g\. 12\.50/).first()).toBeVisible();
  await expect(page).toHaveURL(/\/app\/drivers\/new$/);

  await page.getByRole("button", { name: /^cancel/i }).click();
  await expect(page).toHaveURL(/\/app\/drivers$/);
});

test("user form: a bad email, a short password and a missing driver are each flagged inline", async ({ page }) => {
  await page.goto("/app/users/new");
  await page.waitForTimeout(500);
  await page.getByLabel(/^email/i).fill("not-an-email");
  await page.locator('input[type="password"]').fill("short");
  await page.getByRole("combobox", { name: /^role/i }).click();
  await page.getByRole("option", { name: "Driver" }).click();
  await page.getByRole("button", { name: /^save/i }).click();

  await expect(page.getByText("Enter a valid email address.")).toBeVisible();
  await expect(page.getByText("Must be at least 12 characters.")).toBeVisible();
  await expect(page.getByText("This field is required.")).toHaveCount(1); // the driver
  await expect(page).toHaveURL(/\/app\/users\/new$/);

  await page.getByRole("button", { name: /^cancel/i }).click();
  await expect(page).toHaveURL(/\/app\/users$/);
});
