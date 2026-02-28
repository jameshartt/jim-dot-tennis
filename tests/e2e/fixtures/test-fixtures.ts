import { test as base } from "@playwright/test";
import { loginAsAdmin } from "../helpers/auth";

/**
 * Extended test fixtures that provide auto-login capability.
 *
 * Usage:
 *   import { test, expect } from './fixtures/test-fixtures';
 *
 *   test('admin page loads', async ({ adminPage }) => {
 *     // adminPage is already logged in as admin
 *     await adminPage.goto('/admin/league/players');
 *   });
 */

type TestFixtures = {
  /** A Page instance already authenticated as the test admin user. */
  adminPage: ReturnType<typeof base.extend> extends infer T ? T : never;
};

export const test = base.extend<{ adminPage: import("@playwright/test").Page }>(
  {
    adminPage: async ({ page }, use) => {
      await loginAsAdmin(page);
      await use(page);
    },
  },
);

export { expect } from "@playwright/test";
