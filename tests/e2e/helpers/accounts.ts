import { execFileSync } from "child_process";
import { Browser, BrowserContext, Page, expect } from "@playwright/test";
import { openPasswordForm } from "./auth";
import { VirtualAuthenticator } from "./webauthn";

/** A username that is unique per run, so reruns on the same volume never collide. */
export function uniqueUsername(prefix: string): string {
  return `${prefix}.${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

const DB = process.env.E2E_DB || "/data/tennis.db";

/**
 * Run SQL against the e2e database (the volume the runner shares with the
 * app). Used only for what the UI cannot do: expiring a link without waiting
 * 48 hours, and parking other administrators while the "require passkeys"
 * setting is tested.
 */
export function sql(statement: string): string[] {
  const out = execFileSync("sqlite3", ["-cmd", ".timeout 10000", DB, statement], {
    encoding: "utf8",
  });
  return out.split("\n").filter((l) => l !== "");
}

/** Make a user through the admin users page. Returns the one-time invite link. */
export async function createUser(
  adminPage: Page,
  username: string,
  role: "captain" | "admin" = "captain",
): Promise<string> {
  await adminPage.goto("/admin/league/users");
  const form = adminPage.locator("form[data-user-create]");
  await form.locator("#username").fill(username);
  await form.locator("#role").selectOption(role);
  await form.locator("#create-user-submit").click();
  const link = adminPage.locator("[data-issued-link] #issued-link-url");
  await expect(link).toBeVisible();
  return link.inputValue();
}

/** Ask for a recovery link for an existing user on the admin users page. */
export async function issueRecoveryLink(adminPage: Page, username: string): Promise<string> {
  await adminPage.goto("/admin/league/users");
  adminPage.once("dialog", (d) => d.accept());
  await adminPage
    .locator(`tr[data-username="${username}"] [data-user-recover]`)
    .click();
  const link = adminPage.locator('[data-issued-link][data-link-kind="recovery"] #issued-link-url');
  await expect(link).toBeVisible();
  return link.inputValue();
}

/** Open an invite or recovery link and create a passkey on it. Leaves the page signed in. */
export async function enrolPasskeyFromLink(
  page: Page,
  link: string,
  name = "Test passkey",
  options: { removeOldMethods?: boolean } = {},
): Promise<void> {
  await page.goto(link);
  await expect(page.locator('[data-page="account-link"]')).toBeVisible();
  await page.locator("#link-passkey-name").fill(name);
  if (options.removeOldMethods) {
    await page.locator("#remove_other_methods").check();
  }
  await page.locator("#link-passkey-button").click();
  await page.waitForURL((u) => !u.pathname.startsWith("/account/link/"));
}

/** A brand new signed-out browser context and page, with a virtual authenticator. */
export async function newDevice(
  browser: Browser,
): Promise<{ context: BrowserContext; page: Page; authenticator: VirtualAuthenticator }> {
  const context = await browser.newContext();
  const page = await context.newPage();
  const authenticator = await VirtualAuthenticator.add(page);
  return { context, page, authenticator };
}

/** Sign out through the POST-only logout form. */
export async function signOut(page: Page): Promise<void> {
  await page.locator('form[action="/logout"] [data-testid="logout-button"]').click();
  await expect(page).toHaveURL(/\/login/);
}

/** Sign in with a discoverable passkey from the login page (no username typed). */
export async function signInWithPasskey(page: Page): Promise<void> {
  await page.goto("/login");
  await page.locator("#passkey-signin").click();
  await page.waitForURL((u) => !u.pathname.startsWith("/login"));
}

/** Sign in with a username and password from the login page. */
export async function signInWithPassword(
  page: Page,
  username: string,
  password: string,
): Promise<void> {
  await page.goto("/login");
  await openPasswordForm(page);
  await page.locator("#username").fill(username);
  await page.locator("#password").fill(password);
  await page.locator('form[action="/login"] button[type="submit"]').click();
}
