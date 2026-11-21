import { test, expect } from "./fixtures/test-fixtures";
import {
  createUser,
  enrolPasskeyFromLink,
  issueRecoveryLink,
  newDevice,
  signInWithPassword,
  sql,
  uniqueUsername,
} from "./helpers/accounts";
import { continueAfterPasswordSignIn } from "./helpers/auth";

// Passkey-enabled project: see docker-compose.yml `e2e-auth`.

const INVALID = '[data-page="account-link-invalid"]';

test.describe("Invite and recovery links", () => {
  test("a recovery link ends the account's other sessions and can remove the old methods", async ({
    adminPage,
    browser,
  }) => {
    const username = uniqueUsername("lk.recover");
    const password = "walnut ferry tangerine ledger";
    const invite = await createUser(adminPage, username, "captain");

    // Set up the account with a password through the invite link (the
    // fallback form), so a second browser can hold its own session.
    const first = await browser.newContext();
    const firstPage = await first.newPage();
    const second = await browser.newContext();
    const secondPage = await second.newPage();
    const { context: fresh, page: freshPage } = await newDevice(browser);
    try {
      await firstPage.goto(invite);
      await firstPage.locator("details[data-passkey-fallback] summary").click();
      await firstPage.locator("#new_password").fill(password);
      await firstPage.locator("#new_password_confirm").fill(password);
      await firstPage.locator("#link-password-submit").click();
      await firstPage.waitForURL((u) => !u.pathname.startsWith("/account/link/"));
      await firstPage.goto("/account/signin-methods");
      await expect(firstPage.locator('[data-page="signin-methods"]')).toBeVisible();

      // A second device signed in with the password.
      await signInWithPassword(secondPage, username, password);
      await continueAfterPasswordSignIn(secondPage);
      await secondPage.goto("/account/signin-methods");
      await expect(secondPage.locator('[data-page="signin-methods"]')).toBeVisible();

      // Nothing changes until the recovery link is used.
      const recovery = await issueRecoveryLink(adminPage, username);
      await secondPage.reload();
      await expect(secondPage.locator('[data-page="signin-methods"]')).toBeVisible();

      // Complete it, asking to drop the old ways in.
      await freshPage.goto(recovery);
      await expect(freshPage.locator('[data-page="account-link"]')).toHaveAttribute(
        "data-link-kind",
        "recovery",
      );
      await expect(freshPage.locator("[data-link-remove-choice]")).toBeVisible();
      await enrolPasskeyFromLink(freshPage, recovery, "Replacement", {
        removeOldMethods: true,
      });

      // Both earlier sessions are gone.
      for (const p of [firstPage, secondPage]) {
        await p.goto("/account/signin-methods");
        await expect(p).toHaveURL(/\/login/);
      }

      // The new browser is signed in; the old password is gone.
      await freshPage.goto("/account/signin-methods");
      await expect(freshPage.locator("[data-passkey-list] > li")).toHaveCount(1);
      await expect(freshPage.locator("#password")).toHaveAttribute("data-signin-password", "none");
    } finally {
      await first.close();
      await second.close();
      await fresh.close();
    }
  });

  test("a used link is rejected", async ({ adminPage, browser }) => {
    const username = uniqueUsername("lk.used");
    const invite = await createUser(adminPage, username, "captain");
    const { context, page } = await newDevice(browser);
    const stranger = await browser.newContext();
    try {
      await enrolPasskeyFromLink(page, invite);
      const strangerPage = await stranger.newPage();
      const response = await strangerPage.goto(invite);
      expect(response?.status()).toBe(404);
      await expect(strangerPage.locator(INVALID)).toBeVisible();
      await expect(strangerPage.locator("#link-passkey-button")).toHaveCount(0);
    } finally {
      await context.close();
      await stranger.close();
    }
  });

  test("a cancelled link is rejected", async ({ adminPage, browser }) => {
    const username = uniqueUsername("lk.revoked");
    const invite = await createUser(adminPage, username, "captain");
    const pending = adminPage.locator(`[data-pending-links] tr[data-link-username="${username}"]`);
    await expect(pending).toHaveCount(1);
    adminPage.once("dialog", (d) => d.accept());
    await pending.locator("[data-link-revoke]").click();
    await expect(
      adminPage.locator(`[data-pending-links] tr[data-link-username="${username}"]`),
    ).toHaveCount(0);

    const stranger = await browser.newContext();
    try {
      const p = await stranger.newPage();
      await p.goto(invite);
      await expect(p.locator(INVALID)).toBeVisible();
    } finally {
      await stranger.close();
    }
  });

  test("an expired link is rejected", async ({ adminPage, browser }) => {
    const username = uniqueUsername("lk.expired");
    const invite = await createUser(adminPage, username, "captain");
    // Cannot wait 48 hours: move the expiry into the past in the e2e database.
    sql(
      `UPDATE account_links SET expires_at = '2000-01-01 00:00:00+00:00' ` +
        `WHERE user_id = (SELECT id FROM users WHERE username = '${username}')`,
    );
    const stranger = await browser.newContext();
    try {
      const p = await stranger.newPage();
      await p.goto(invite);
      await expect(p.locator(INVALID)).toBeVisible();
    } finally {
      await stranger.close();
    }
  });
});
