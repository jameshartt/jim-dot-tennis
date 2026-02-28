import { test as base } from "@playwright/test";
import { loginAsAdmin } from "../helpers/auth";

/**
 * Extended test fixtures that provide auto-login capability.
 *
 * Uses the storage state saved by global-setup.ts, with a fallback to
 * direct login if the stored session is no longer valid. This prevents
 * flakiness from SQLite locking invalidating sessions.
 *
 * Usage:
 *   import { test, expect } from './fixtures/test-fixtures';
 *
 *   test('admin page loads', async ({ adminPage }) => {
 *     // adminPage is already logged in as admin
 *     await adminPage.goto('/admin/league/players');
 *   });
 */

export const test = base.extend<{ adminPage: import("@playwright/test").Page }>(
  {
    adminPage: async ({ browser }, use) => {
      const context = await browser.newContext({
        storageState: "auth-state.json",
      });
      const page = await context.newPage();

      // Verify the stored session is still valid
      const response = await page.goto("/admin/league/dashboard");
      if (page.url().includes("/login")) {
        // Session expired or was invalidated — login directly
        await loginAsAdmin(page);
      }

      await use(page);
      await context.close();
    },
  },
);

export { expect } from "@playwright/test";
