import { readFileSync } from "node:fs";
import path from "node:path";
import { expect, Page, Request } from "@playwright/test";
import { continueAfterPasswordSignIn, openPasswordForm } from "../helpers/auth";

/** The first administrator every run creates through first-run set-up. */
export const ADMIN_USER = "bundle.admin";
export const ADMIN_PASSWORD = "a quiet morning on court seven";

/** The club's tournament code: made from "St Ann's Tennis" by tournament set-up. */
export const CLUB_CODE = "STANNS";

/** The origin every request of a run must stay on. */
export function testOrigin(): string {
  return new URL(process.env.BUNDLE_URL || "https://localhost:18743").host;
}

/**
 * The one-time set-up code the app printed at start-up. The script saves the
 * app's log into BUNDLE_LOG_DIR before the tests run; the code is read at run
 * time and never written anywhere.
 */
export function readSetupCode(): string {
  const file = path.join(process.env.BUNDLE_LOG_DIR || "/bundle-logs", "app.log");
  const matches = [...readFileSync(file, "utf8").matchAll(/SETUP CODE: ([A-Z0-9-]+)/g)];
  if (matches.length === 0) throw new Error(`no "SETUP CODE:" banner in ${file}`);
  return matches[matches.length - 1][1];
}

/**
 * First-run set-up of an empty instance, as in the fresh-install suite: the
 * code from the log, the first administrator, then the home club (St Ann's,
 * read from the saved BHPLTA pages served inside the test project). Ends on
 * the done page with the administrator signed in.
 */
export async function firstRunSetUp(page: Page): Promise<void> {
  await page.goto("/setup");
  await page.fill("#code", readSetupCode());
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL(/\/setup\/admin$/);
  await page.fill("#username", ADMIN_USER);
  await page.fill("#password", ADMIN_PASSWORD);
  await page.fill("#password_confirm", ADMIN_PASSWORD);
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL(/\/setup\/club$/);
  await page.check('input[name="club"][value="st-anns"]');
  await page.click('button[type="submit"]:has-text("Use this club")');
  await expect(page).toHaveURL(/\/setup\/done$/, { timeout: 90_000 });
  await expect(page.locator('[data-setup-step="done"]')).toBeVisible();
}

/** Signs in at /login with a password, carrying on past the "add a passkey?" offer. */
export async function signInWithPassword(page: Page, username: string, password: string): Promise<void> {
  await page.goto("/login");
  await openPasswordForm(page);
  await page.fill('input[name="username"]', username);
  await page.fill('input[name="password"]', password);
  await page.click('form[action="/login"] button[type="submit"]');
  await continueAfterPasswordSignIn(page);
}

export async function signOut(page: Page): Promise<void> {
  await page.locator('[data-testid="logout-button"]').first().click();
  await expect(page).toHaveURL(/\/login/);
}

/** Records the host of every request a page makes (to prove nothing leaves the test origin). */
export function recordHosts(page: Page): Set<string> {
  const hosts = new Set<string>();
  page.on("request", (r: Request) => hosts.add(new URL(r.url()).host));
  return hosts;
}

/** TMX's two session tokens, as stored in the browser. */
export async function tmxTokens(page: Page): Promise<{ access: string | null; refresh: string | null }> {
  return page.evaluate(() => ({
    access: localStorage.getItem("tmxToken"),
    refresh: localStorage.getItem("tmxRefreshToken"),
  }));
}

/**
 * GET /api/courthive/auth/me from inside the page with TMX's own token: the
 * proof that the hand-over gave a working session. Returns the status and body.
 */
export async function authMe(page: Page): Promise<{ status: number; body: any }> {
  return page.evaluate(async () => {
    const token = localStorage.getItem("tmxToken");
    const r = await fetch("/api/courthive/auth/me", { headers: { Authorization: `Bearer ${token}` } });
    let body: any = null;
    try {
      body = await r.json();
    } catch {
      /* not JSON */
    }
    return { status: r.status, body };
  });
}

/**
 * Picks a day in one of TMX's date pickers by clicking in the calendar (typing
 * into the field does not work: Enter picks "today"). `nextMonths` presses the
 * calendar's next-month button first. The picker opens on the month of the
 * date already chosen in the other field, hence the explicit count.
 */
export async function pickDate(page: Page, index: number, day: number, nextMonths: number): Promise<string> {
  const field = page.locator("input.datepicker-input").nth(index);
  await field.click();
  for (let i = 0; i < nextMonths; i++) {
    await page.locator(".datepicker-picker .next-btn").locator("visible=true").first().click();
  }
  await page
    .locator(".datepicker-cell.day:not(.prev):not(.next)", { hasText: new RegExp(`^${day}$`) })
    .locator("visible=true")
    .first()
    .click();
  return field.inputValue();
}
