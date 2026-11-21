import { test, expect } from "@playwright/test";
import { VirtualAuthenticator } from "./helpers/webauthn";

// Passkey-enabled project. scripts/test-e2e-auth.sh runs
// `jim-dot-tennis user create <name>` and `user reset <name>` inside the app
// container (the Playwright container has no Docker access) and passes the
// printed recovery link in E2E_CLI_RESET_LINK and the account in E2E_CLI_USER.
// The link works once, so this test must not be retried.

test.describe.configure({ retries: 0 });

test("the recovery link printed by `jim-dot-tennis user reset` works once", async ({ browser }) => {
  const link = process.env.E2E_CLI_RESET_LINK;
  const username = process.env.E2E_CLI_USER;
  test.skip(!link || !username, "run through scripts/test-e2e-auth.sh (make test-e2e-auth)");

  const context = await browser.newContext();
  const page = await context.newPage();
  await VirtualAuthenticator.add(page);
  try {
    await page.goto(link!);
    await expect(page.locator('[data-page="account-link"]')).toHaveAttribute(
      "data-link-kind",
      "recovery",
    );
    await expect(page.locator("[data-link-username]")).toHaveText(username!);
    await page.locator("#link-passkey-name").fill("CLI recovered");
    await page.locator("#link-passkey-button").click();
    await page.waitForURL((u) => !u.pathname.startsWith("/account/link/"));
    await page.goto("/account/signin-methods");
    await expect(page.locator('[data-page="signin-methods"]')).toContainText(username!);
    await expect(page.locator("[data-passkey-list] > li")).toHaveCount(1);

    // Single use.
    const stranger = await browser.newContext();
    const p = await stranger.newPage();
    await p.goto(link!);
    await expect(p.locator('[data-page="account-link-invalid"]')).toBeVisible();
    await stranger.close();
  } finally {
    await context.close();
  }
});
