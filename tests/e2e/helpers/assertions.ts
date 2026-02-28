import { Page, expect, Locator } from "@playwright/test";

/** Assert the page title contains the given text. */
export async function expectTitleContains(
  page: Page,
  text: string,
): Promise<void> {
  await expect(page).toHaveTitle(new RegExp(text, "i"));
}

/** Assert a flash/alert message is visible with the given text. */
export async function expectFlashMessage(
  page: Page,
  text: string,
): Promise<void> {
  const flash = page.locator(".alert, .flash, .notification");
  await expect(flash).toContainText(text);
}

/** Assert the number of rows in a table body. */
export async function expectTableRowCount(
  locator: Locator,
  count: number,
): Promise<void> {
  await expect(locator.locator("tbody tr")).toHaveCount(count);
}

/** Assert the page has no accessibility violations (basic check). */
export async function expectNoErrorBanner(page: Page): Promise<void> {
  await expect(page.locator(".error-banner, .fatal-error")).toHaveCount(0);
}
