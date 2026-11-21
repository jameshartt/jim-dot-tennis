import { test, expect } from "./fixtures/test-fixtures";
import {
  addSecondPasskey,
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
      await expect(page.locator("[data-passkey-remove-open]")).toHaveCount(0);
      // Being signed in is not enough to add a passkey: there is no Add
      // button until it is confirmed that this is the account's owner.
      await expect(page.locator("#passkey-add")).toHaveCount(0);

      // Confirm with the first device, then add a second one.
      const { second, firstHeld } = await addSecondPasskey(page, authenticator, "Second device");
      await expect(items).toHaveCount(2);
      await expect(page.locator("[data-passkey-last-method]")).toHaveCount(0);
      expect(await second.credentialCount()).toBe(1);

      // Rename the second one (no confirmation needed for a name).
      const secondItem = items.filter({ hasText: "Second device" });
      await secondItem.locator('input[name="name"]').fill("Renamed device");
      await secondItem.locator("[data-passkey-rename]").click();
      await expect(items.filter({ hasText: "Renamed device" })).toHaveCount(1);
      await expect(items.filter({ hasText: "Second device" })).toHaveCount(0);

      // Removing one needs a confirmation too, and the passkey added a moment
      // ago cannot give it: only one the account already had.
      const firstItem = items.filter({ hasText: "First device" });
      await firstItem.locator("[data-passkey-remove-open]").click();
      await expect(firstItem.locator("[data-passkey-remove]")).toHaveCount(0);
      await firstItem.locator("[data-passkey-reauth]").click();
      await expect(page.locator("#passkey-status")).toContainText("only just added");
      await expect(items).toHaveCount(2);

      // The first device can.
      const secondHeld = await second.putAway();
      const firstAgain = await VirtualAuthenticator.restore(page, firstHeld);
      await page.reload();
      await firstItem.locator("[data-passkey-remove-open]").click();
      await firstItem.locator("[data-passkey-reauth]").click();
      await expect(page.locator("[data-signin-notice]")).toBeVisible();
      await firstItem.locator("[data-passkey-remove-open]").click();
      await firstItem.locator("[data-passkey-remove]").click();
      await expect(items).toHaveCount(1);
      await expect(items.filter({ hasText: "Renamed device" })).toHaveCount(1);
      // What is left is the last method.
      await expect(page.locator("[data-passkey-last-method]")).toBeVisible();

      // The remaining passkey still signs in.
      await firstAgain.remove();
      await VirtualAuthenticator.restore(page, secondHeld);
      await signOut(page);
      await signInWithPasskey(page);
      await page.goto("/account/signin-methods");
      await expect(methods).toBeVisible();
    } finally {
      await context.close();
    }
  });

  test("opening the remove box without confirming keeps the passkey", async ({ adminPage, browser }) => {
    const username = uniqueUsername("pk.cancel");
    const link = await createUser(adminPage, username, "captain");
    const { context, page, authenticator } = await newDevice(browser);
    try {
      await enrolPasskeyFromLink(page, link, "Only device");
      await addSecondPasskey(page, authenticator, "Extra");
      const items = page.locator("[data-passkey-list] > li");
      await expect(items).toHaveCount(2);
      // No Remove button is offered until ownership is confirmed, and a
      // request without proof changes nothing.
      await items.first().locator("[data-passkey-remove-open]").click();
      await expect(items.first().locator("[data-passkey-remove]")).toHaveCount(0);
      const id = await items.first().getAttribute("data-passkey-id");
      const response = await page.request.post(`/account/passkeys/${id}/delete`, {
        form: {},
        headers: { Origin: new URL(page.url()).origin },
        maxRedirects: 0,
      });
      expect(response.status()).toBe(303);
      expect(response.headers()["location"]).toContain("error=reauth");
      await page.reload();
      await expect(items).toHaveCount(2);
    } finally {
      await context.close();
    }
  });

  test("a password-only account adds a passkey by typing its password; adding one signs other devices out", async ({
    adminPage,
    browser,
  }) => {
    const username = uniqueUsername("pk.first");
    const password = "amber thistle canyon rowboat";
    const link = await createUser(adminPage, username, "captain");
    const { context, page } = await newDevice(browser);
    const other = await browser.newContext();
    try {
      // Set the account up with a password only.
      await page.goto(link);
      await page.locator("details[data-passkey-fallback] summary").click();
      await page.locator("#new_password").fill(password);
      await page.locator("#new_password_confirm").fill(password);
      await page.locator("#link-password-submit").click();
      await page.waitForURL((u) => !u.pathname.startsWith("/account/link/"));

      // A second device signed in to the same account.
      const otherPage = await other.newPage();
      await signInWithPassword(otherPage, username, password);
      await otherPage.waitForURL((u) => u.pathname !== "/login");
      await otherPage.goto("/account/signin-methods");
      await expect(otherPage.locator('[data-page="signin-methods"]')).toBeVisible();

      await page.goto("/account/signin-methods");
      await expect(page.locator("#sessions")).toHaveAttribute("data-other-sessions", "1");
      const items = page.locator("[data-passkey-list] > li");
      // Without the password: refused, nothing added.
      await page.locator("#new-passkey-name").fill("My phone");
      await page.locator("#passkey-add").click();
      await expect(page.locator("#passkey-status")).toContainText("confirm it's you");
      await expect(items).toHaveCount(0);
      // With a wrong one: refused.
      await page.locator("#passkey-add-current-password").fill("not the password at all");
      await page.locator("#passkey-add").click();
      await expect(page.locator("#passkey-status")).toContainText("not right");
      await expect(items).toHaveCount(0);
      // With the right one: added.
      await page.locator("#passkey-add-current-password").fill(password);
      await page.locator("#passkey-add").click();
      await expect(items).toHaveCount(1);

      // The other device was signed out by that; this one was not.
      await expect(page.locator("#sessions")).toHaveAttribute("data-other-sessions", "0");
      await otherPage.goto("/account/signin-methods");
      await expect(otherPage).toHaveURL(/\/login/);

      // "Sign out everywhere else" does the same on request.
      await signInWithPassword(otherPage, username, password);
      await otherPage.waitForURL((u) => u.pathname !== "/login");
      await page.reload();
      await expect(page.locator("#sessions")).toHaveAttribute("data-other-sessions", "1");
      await page.locator("[data-signout-elsewhere]").click();
      await expect(page.locator("[data-signin-notice]")).toBeVisible();
      await expect(page.locator("#sessions")).toHaveAttribute("data-other-sessions", "0");
      await otherPage.goto("/account/signin-methods");
      await expect(otherPage).toHaveURL(/\/login/);
    } finally {
      await context.close();
      await other.close();
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
      await page.locator("#password-confirm-passkey").click();
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
        await page.locator("#password-confirm-passkey").click();
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
        await page.locator("#password-confirm-passkey").click();
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

      // Remove it: back to passkey only. That needs the current password (or
      // a passkey confirmation) as well.
      await page.goto("/account/signin-methods");
      const removeBox = page.locator("[data-password-remove-box]");
      await removeBox.locator("[data-password-remove-open]").click();
      await removeBox.locator("[data-proof-password]").fill("not the right password");
      await removeBox.locator("[data-password-remove]").click();
      await expect(page.locator("[data-password-error]")).toBeVisible();
      await expect(page.locator("#password")).toHaveAttribute("data-signin-password", "set");
      await removeBox.locator("[data-password-remove-open]").click();
      await removeBox.locator("[data-proof-password]").fill(PASSWORD_2);
      await removeBox.locator("[data-password-remove]").click();
      await expect(page.locator("#password")).toHaveAttribute("data-signin-password", "none");
    } finally {
      await context.close();
    }
  });
});
