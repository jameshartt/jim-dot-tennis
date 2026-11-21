import { test, expect } from "@playwright/test";
import {
  createUser,
  enrolPasskeyFromLink,
  newDevice,
  signInWithPasskey,
  signInWithPassword,
  signOut,
  sql,
  uniqueUsername,
} from "./helpers/accounts";
import { loginAsAdmin } from "./helpers/auth";

// Passkey-enabled project: see docker-compose.yml `e2e-auth`.
// "Require passkeys for administrators" is app-wide, so this file turns it
// off again in afterAll (and re-activates any administrator it parked); the
// auth project runs with one worker, so nothing else is running meanwhile.

let parked: string[] = [];

function restore() {
  sql(`UPDATE app_settings SET value = 'false' WHERE key = 'require_admin_passkeys'`);
  if (parked.length) {
    sql(`UPDATE users SET is_active = 1 WHERE id IN (${parked.join(",")})`);
    parked = [];
  }
}

test.afterAll(() => restore());

test("require passkeys for administrators: refused until every admin has one, then password sign-in stops working", async ({
  browser,
}) => {
  const username = uniqueUsername("adm.pk");
  const password = "granite orchard velvet kettle";

  // The seeded admin creates a second administrator, who gets a passkey and a password.
  const seeded = await browser.newContext();
  const seededPage = await seeded.newPage();
  await loginAsAdmin(seededPage);
  const invite = await createUser(seededPage, username, "admin");
  await seeded.close();

  const { context, page } = await newDevice(browser);
  try {
    await enrolPasskeyFromLink(page, invite, "Admin laptop");
    await page.goto("/account/signin-methods");
    await page.locator("#password-confirm-passkey").click();
    const form = page.locator('form[data-password-form="add"]');
    await expect(page.locator("[data-password-confirmed]")).toBeVisible();
    await form.locator("#new_password").fill(password);
    await form.locator("#new_password_confirm").fill(password);
    await form.locator("[data-password-submit]").click();
    await expect(page.locator("[data-password-notice]")).toBeVisible();

    // Other active administrators (the seeded one) have no passkey: refused, and named.
    await page.goto("/admin/league/settings");
    const section = page.locator("#signin[data-admin-passkeys]");
    await expect(section).toHaveAttribute("data-admin-passkeys", "off");
    await expect(section.locator("[data-admin-passkeys-missing]")).toContainText("testadmin");
    await section.locator("#admin-passkeys-on").click();
    await expect(page.locator("[data-admin-passkeys-error]")).toBeVisible();
    await expect(page.locator("#signin")).toHaveAttribute("data-admin-passkeys", "off");

    // Park every other active administrator (only the database can do that
    // without their own passkeys), so the setting can be switched on.
    const me = sql(`SELECT id FROM users WHERE username = '${username}'`)[0];
    parked = sql(`SELECT id FROM users WHERE role = 'admin' AND is_active = 1 AND id != ${me}`);
    sql(`UPDATE users SET is_active = 0 WHERE id IN (${parked.join(",")})`);

    await page.goto("/admin/league/settings");
    await page.locator("#admin-passkeys-on").click();
    await expect(page.locator("#signin")).toHaveAttribute("data-admin-passkeys", "on");

    // A password no longer signs this administrator in: the same generic
    // error as a wrong password.
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await signInWithPassword(otherPage, username, password);
    await expect(otherPage).toHaveURL(/\/login/);
    await expect(otherPage.locator(".alert.alert-danger:not([hidden])")).toContainText("invalid username or password");
    await other.close();

    // The passkey still does.
    await page.goto("/account/signin-methods"); // has the sign-out form
    await signOut(page);
    await signInWithPasskey(page);
    await page.goto("/admin/league/settings");
    await expect(page.locator("#signin")).toHaveAttribute("data-admin-passkeys", "on");

    // Turn it off again; passwords work for administrators once more.
    await page.locator("#admin-passkeys-off").click();
    await expect(page.locator("#signin")).toHaveAttribute("data-admin-passkeys", "off");
  } finally {
    await context.close();
    restore();
  }
});
