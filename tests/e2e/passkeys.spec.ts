import { test, expect } from "./fixtures/test-fixtures";
import {
  createUser,
  enrolPasskeyFromLink,
  newDevice,
  signInWithPasskey,
  signInWithPassword,
  signOut,
  uniqueUsername,
} from "./helpers/accounts";
import { VirtualAuthenticator } from "./helpers/webauthn";

// Passkey-enabled project: see docker-compose.yml `e2e-auth`.

test.describe("Passkeys from an invite link", () => {
  test("enrol a passkey from an invite link, then sign in with it", async ({
    adminPage,
    browser,
  }) => {
    const username = uniqueUsername("pk.invite");
    const link = await createUser(adminPage, username, "captain");
    const { context, page, authenticator } = await newDevice(browser);
    try {
      // The link page shows who it is for before anything changes.
      await page.goto(link);
      await expect(page.locator('[data-page="account-link"]')).toHaveAttribute(
        "data-link-kind",
        "invite",
      );
      await expect(page.locator("[data-link-username]")).toHaveText(username);
      await expect(page.locator("[data-link-role]")).toHaveText("captain");

      await page.locator("#link-passkey-name").fill("Pixel (virtual)");
      await page.locator("#link-passkey-button").click();
      await page.waitForURL((u) => !u.pathname.startsWith("/account/link/"));

      // Signed in as the new captain.
      await page.goto("/admin/league/dashboard");
      await expect(page.locator('[data-testid="dashboard-title"]')).toBeVisible();
      await expect(page.locator('[data-testid="user-role"]')).toHaveAttribute("data-role", "captain");
      expect(await authenticator.credentialCount()).toBe(1);

      // Sign out; the session is gone.
      await signOut(page);
      await page.goto("/admin/league/dashboard");
      await expect(page).toHaveURL(/\/login/);

      // Discoverable sign-in: no username typed.
      await signInWithPasskey(page);
      await page.goto("/admin/league/dashboard");
      await expect(page.locator('[data-testid="dashboard-title"]')).toBeVisible();
      await expect(page.locator('[data-testid="user-role"]')).toHaveAttribute("data-role", "captain");
    } finally {
      await context.close();
    }
  });
});

test.describe("Managing passkeys", () => {
  test("add a second passkey, rename it, remove one; the last one cannot be removed", async ({
    adminPage,
    browser,
  }) => {
    const username = uniqueUsername("pk.manage");
    const link = await createUser(adminPage, username, "captain");
    const { context, page, authenticator } = await newDevice(browser);
    try {
      await enrolPasskeyFromLink(page, link, "First device");
      await page.goto("/account/signin-methods");
      const methods = page.locator('[data-page="signin-methods"]');
      await expect(methods).toBeVisible();
      const items = page.locator("[data-passkey-list] > li");
      await expect(items).toHaveCount(1);
      // The only way to sign in cannot be removed.
      await expect(page.locator("[data-passkey-last-method]")).toBeVisible();
      await expect(page.locator("[data-passkey-remove]")).toHaveCount(0);

      // A second device: the first one already holds a credential for this
      // account, so it would refuse; use a clean authenticator.
      await authenticator.remove();
      const second = await VirtualAuthenticator.add(page);
      await page.locator("#new-passkey-name").fill("Second device");
      await page.locator("#passkey-add").click();
      await expect(items).toHaveCount(2);
      await expect(page.locator("[data-passkey-last-method]")).toHaveCount(0);
      await expect(items.filter({ hasText: "Second device" })).toHaveCount(1);
      expect(await second.credentialCount()).toBe(1);

      // Rename the second one.
      const secondItem = items.filter({ hasText: "Second device" });
      await secondItem.locator('input[name="name"]').fill("Renamed device");
      await secondItem.locator("[data-passkey-rename]").click();
      await expect(items.filter({ hasText: "Renamed device" })).toHaveCount(1);
      await expect(items.filter({ hasText: "Second device" })).toHaveCount(0);

      // Remove the first one (the page asks first).
      const firstItem = items.filter({ hasText: "First device" });
      page.once("dialog", (d) => d.accept());
      await firstItem.locator("[data-passkey-remove]").click();
      await expect(items).toHaveCount(1);
      await expect(items.filter({ hasText: "Renamed device" })).toHaveCount(1);
      // What is left is the last method.
      await expect(page.locator("[data-passkey-last-method]")).toBeVisible();

      // The remaining passkey still signs in.
      await signOut(page);
      await signInWithPasskey(page);
      await page.goto("/account/signin-methods");
      await expect(methods).toBeVisible();
    } finally {
      await context.close();
    }
  });

  test("declining the remove confirmation keeps the passkey", async ({ adminPage, browser }) => {
    const username = uniqueUsername("pk.cancel");
    const link = await createUser(adminPage, username, "captain");
    const { context, page, authenticator } = await newDevice(browser);
    try {
      await enrolPasskeyFromLink(page, link, "Only device");
      await page.goto("/account/signin-methods");
      await authenticator.remove();
      await VirtualAuthenticator.add(page);
      await page.locator("#new-passkey-name").fill("Extra");
      await page.locator("#passkey-add").click();
      const items = page.locator("[data-passkey-list] > li");
      await expect(items).toHaveCount(2);
      page.once("dialog", (d) => d.dismiss());
      await items.first().locator("[data-passkey-remove]").click();
      await page.reload();
      await expect(items).toHaveCount(2);
    } finally {
      await context.close();
    }
  });
});

test.describe("Password as a fallback", () => {
  const PASSWORD_1 = "violet harbour lantern ferry";
  const PASSWORD_2 = "plum ocean walnut cobblestone";

  test("add, change and remove a password; weak passwords are refused", async ({
    adminPage,
    browser,
  }) => {
    const username = uniqueUsername("pk.password");
    const link = await createUser(adminPage, username, "captain");
    const { context, page } = await newDevice(browser);
    try {
      await enrolPasskeyFromLink(page, link, "Phone");
      await page.goto("/account/signin-methods");
      const section = page.locator("#password");
      await expect(section).toHaveAttribute("data-signin-password", "none");
      await expect(page.locator("[data-password-remove]")).toHaveCount(0);

      // Adding a password first needs a passkey confirmation.
      await page.locator("[data-passkey-reauth]").click();
      const addForm = page.locator('form[data-password-form="add"]');
      await expect(page.locator("[data-password-confirmed]")).toBeVisible();

      // Policy: too short is refused. (The browser's own minlength check would
      // stop the form first, so switch the form's validation off.)
      await addForm.evaluate((f) => ((f as HTMLFormElement).noValidate = true));
      await addForm.locator("#new_password").fill("short1");
      await addForm.locator("#new_password_confirm").fill("short1");
      await addForm.locator("[data-password-submit]").click();
      await expect(page.locator("[data-password-error]")).toBeVisible();
      await expect(section).toHaveAttribute("data-signin-password", "none");

      // A well-known password is refused too, even though it is long enough.
      let form = page.locator('form[data-password-form="add"]');
      if (!(await page.locator("[data-password-confirmed]").isVisible())) {
        await page.locator("[data-passkey-reauth]").click();
        await expect(page.locator("[data-password-confirmed]")).toBeVisible();
      }
      form = page.locator('form[data-password-form="add"]');
      await form.locator("#new_password").fill("123456789012");
      await form.locator("#new_password_confirm").fill("123456789012");
      await form.locator("[data-password-submit]").click();
      await expect(page.locator("[data-password-error]")).toBeVisible();
      await expect(section).toHaveAttribute("data-signin-password", "none");

      // A good password is accepted.
      if (!(await page.locator("[data-password-confirmed]").isVisible())) {
        await page.locator("[data-passkey-reauth]").click();
        await expect(page.locator("[data-password-confirmed]")).toBeVisible();
      }
      form = page.locator('form[data-password-form="add"]');
      await form.locator("#new_password").fill(PASSWORD_1);
      await form.locator("#new_password_confirm").fill(PASSWORD_1);
      await form.locator("[data-password-submit]").click();
      await expect(page.locator("[data-password-notice]")).toBeVisible();
      await expect(section).toHaveAttribute("data-signin-password", "set");

      // The password signs in from another device.
      const other = await browser.newContext();
      const otherPage = await other.newPage();
      await signInWithPassword(otherPage, username, PASSWORD_1);
      await otherPage.goto("/admin/league/dashboard");
      await expect(otherPage.locator('[data-testid="dashboard-title"]')).toBeVisible();
      await other.close();

      // Change it: confirm with the current password.
      await page.goto("/account/signin-methods");
      const change = page.locator('form[data-password-form="change"]');
      await change.locator("#current_password").fill(PASSWORD_1);
      await change.locator("#new_password").fill(PASSWORD_2);
      await change.locator("#new_password_confirm").fill(PASSWORD_2);
      await change.locator("[data-password-submit]").click();
      await expect(page.locator("[data-password-notice]")).toBeVisible();

      // The new one works from a fresh device.
      const third = await browser.newContext();
      const thirdPage = await third.newPage();
      await signInWithPassword(thirdPage, username, PASSWORD_2);
      await thirdPage.goto("/admin/league/dashboard");
      await expect(thirdPage.locator('[data-testid="dashboard-title"]')).toBeVisible();
      await third.close();

      // Remove it: back to passkey only.
      await page.goto("/account/signin-methods");
      page.once("dialog", (d) => d.accept());
      await page.locator("[data-password-remove]").click();
      await expect(page.locator("#password")).toHaveAttribute("data-signin-password", "none");
    } finally {
      await context.close();
    }
  });
});
