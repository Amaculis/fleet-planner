// Shared helpers for the e2e suite.
//
// trackConsoleErrors(page) is the one mechanism behind almost every assertion here:
// every bug found in this app's SPA this session (missing CSS modules didn't error,
// but the CSP eval violation, the CSP inline-style violations, and the invalid
// dynamic icon imports all did) showed up as a console error in a real browser.
// Collecting those and asserting the list is empty is what actually would have
// caught them automatically, instead of needing a human to paste console output.
export function trackConsoleErrors(page) {
  const errors = [];
  page.on("console", (msg) => {
    if (msg.type() === "error") errors.push(msg.text());
  });
  page.on("pageerror", (err) => errors.push(err.message));
  return errors;
}

export async function login(page, email, password) {
  await page.goto("/app/login");
  await page.getByRole("textbox", { name: /email/i }).fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.getByRole("button", { name: /sign in/i }).click();
  await page.waitForURL(/\/app\/(dashboard)?$/);
}

export const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL;
export const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD;
