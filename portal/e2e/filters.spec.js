import { test, expect } from "@playwright/test";
import { trackConsoleErrors } from "./support.js";

test.skip(!process.env.E2E_ADMIN_EMAIL, "E2E_ADMIN_EMAIL not set");

async function createBus(page, plate, status) {
  return page.evaluate(
    async ({ plate, status }) => {
      const me = await fetch("/api/auth/me").then((r) => r.json());
      const resp = await fetch("/api/buses", {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-CSRF-Token": me.csrfToken },
        body: JSON.stringify({ plate, model: "Filter Test", seats: 40, status }),
      });
      return resp.status;
    },
    { plate, status }
  );
}

// The lists' filters are LxFilters panels (collapsible, labelled fields, the component's
// own Apply/Clear buttons) sitting above the grid — not dropdowns in the grid toolbar.
test("bus list filters: collapsed by default, Apply narrows the grid, Clear restores it", async ({ page }) => {
  const suffix = Date.now() % 100000;
  const maintenancePlate = `FM${suffix}`;
  const activePlate = `FA${suffix}`;

  await page.goto("/app/");
  expect(await createBus(page, maintenancePlate, "maintenance")).toBe(201);
  expect(await createBus(page, activePlate, "active")).toBe(201);

  const errors = trackConsoleErrors(page);
  await page.goto("/app/buses");
  await page.waitForTimeout(500);

  // English UI must not show LxFilters' built-in Latvian defaults.
  await expect(page.getByText(/Atlasīt|Notīrīt|Filtri/)).toHaveCount(0);

  // Hidden until expanded: the labelled field isn't reachable yet.
  await expect(page.getByRole("combobox", { name: "Status" })).toBeHidden();
  await page.getByRole("button", { name: /^filters/i }).click();
  await expect(page.getByRole("combobox", { name: "Status" })).toBeVisible();

  // Nothing is applied until the Apply button is pressed (the picker only edits a draft).
  await page.getByRole("combobox", { name: "Status" }).click();
  await page.getByRole("option", { name: "In maintenance" }).click();
  await expect(page.getByText(activePlate)).toBeVisible();

  await page.getByRole("button", { name: /^apply$/i }).click();
  await expect(page.getByText(maintenancePlate)).toBeVisible();
  await expect(page.getByText(activePlate)).toHaveCount(0);
  await expect(page.getByRole("button", { name: /filters applied/i })).toBeVisible();

  await page.getByRole("button", { name: /^clear$/i }).click();
  await expect(page.getByText(maintenancePlate)).toBeVisible();
  await expect(page.getByText(activePlate)).toBeVisible();
  await expect(page.getByRole("button", { name: /filters applied/i })).toHaveCount(0);

  expect(errors, `console errors using the bus filters:\n${errors.join("\n")}`).toEqual([]);
});

test("every list has a labelled, collapsed filter panel", async ({ page }) => {
  const errors = trackConsoleErrors(page);
  const expected = {
    buses: ["Status"],
    drivers: ["Active"],
    users: ["Role", "Active"],
    trips: ["Status", "Payment status"],
  };
  for (const [list, labels] of Object.entries(expected)) {
    await page.goto(`/app/${list}`);
    await page.waitForTimeout(500);
    for (const label of labels) {
      await expect(page.getByRole("combobox", { name: label, exact: true }), `${list}: ${label} hidden`).toBeHidden();
    }
    await page.getByRole("button", { name: /^filters/i }).click();
    for (const label of labels) {
      await expect(page.getByRole("combobox", { name: label, exact: true }), `${list}: ${label} visible`).toBeVisible();
    }
  }
  expect(errors, `console errors opening the filter panels:\n${errors.join("\n")}`).toEqual([]);
});
