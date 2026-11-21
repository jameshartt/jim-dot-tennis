import { Page, expect } from "@playwright/test";

const TEST_ADMIN_USERNAME = "testadmin";
const TEST_ADMIN_PASSWORD = "testpassword123";

/**
 * Where passkeys work the login page folds the password form into a
 * <details>; where they do not (the main suite's http://app-e2e:8080) it is
 * open already. Open it if it is closed.
 */
export async function openPasswordForm(page: Page): Promise<void> {
  const details = page.locator("details[data-passkey-fallback]");
  if (await details.count()) {
    const open = await details.evaluate((el) => (el as HTMLDetailsElement).open);
    if (!open) await details.locator("summary").click();
  }
}

/**
 * After a successful password sign-in, where passkeys work and the account has
 * none, the app offers to add one first (sign-in methods page with a Continue
 * link). Carry on to where the sign-in was heading.
 */
export async function continueAfterPasswordSignIn(page: Page): Promise<void> {
  await page.waitForURL((u) => u.pathname !== "/login");
  if (new URL(page.url()).pathname === "/account/signin-methods") {
    const next = page.locator("[data-signin-continue]");
    if (await next.count()) await next.click();
  }
}

/**
 * Log in as the seeded admin user and verify we land on the dashboard.
 */
export async function loginAsAdmin(page: Page): Promise<void> {
  await page.goto("/login");
  await openPasswordForm(page);
  await page.fill('input[name="username"]', TEST_ADMIN_USERNAME);
  await page.fill('input[name="password"]', TEST_ADMIN_PASSWORD);
  await page.click('button[type="submit"]');
  await continueAfterPasswordSignIn(page);
  await page.waitForURL("**/admin/league/dashboard");
  await expect(page).toHaveURL(/\/admin\/league\/dashboard/);
}

/**
 * Attempt login with given credentials (does not assert outcome).
 */
export async function loginWith(
  page: Page,
  username: string,
  password: string,
): Promise<void> {
  await page.goto("/login");
  await openPasswordForm(page);
  await page.fill('input[name="username"]', username);
  await page.fill('input[name="password"]', password);
  await page.click('button[type="submit"]');
}

/** Log in as the seeded captain (testcaptain, see fixtures/seed.sql). */
export async function loginAsCaptain(page: Page): Promise<void> {
  await loginWith(page, "testcaptain", TEST_ADMIN_PASSWORD);
  await continueAfterPasswordSignIn(page);
  await page.waitForURL("**/admin/league/dashboard");
}
