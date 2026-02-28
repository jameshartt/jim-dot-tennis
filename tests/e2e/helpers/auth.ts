import { Page, expect } from "@playwright/test";

const TEST_ADMIN_USERNAME = "testadmin";
const TEST_ADMIN_PASSWORD = "testpassword123";

/**
 * Log in as the seeded admin user and verify we land on the dashboard.
 */
export async function loginAsAdmin(page: Page): Promise<void> {
  await page.goto("/login");
  await page.fill('input[name="username"]', TEST_ADMIN_USERNAME);
  await page.fill('input[name="password"]', TEST_ADMIN_PASSWORD);
  await page.click('button[type="submit"]');
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
  await page.fill('input[name="username"]', username);
  await page.fill('input[name="password"]', password);
  await page.click('button[type="submit"]');
}
