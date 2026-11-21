import { test, expect } from "@playwright/test";
import { readSetupCode, sqlite } from "./helpers/fresh-install";

/**
 * Demo data on its own empty instance (app-fresh-demo): set up the first
 * administrator, load the demo data from the club step, see the banner, clear
 * it and land back at club selection with only the administrator left.
 */

const DEMO_URL = process.env.FRESH_DEMO_URL || "http://app-fresh-demo:8080";
const DEMO_DB = "/demo-data/tennis.db";
const ADMIN_PASSWORD = "another long passphrase here";

test.describe.configure({ mode: "serial" });

test("demo data: load, banner, clear, back to club selection", async ({ browser }) => {
  const context = await browser.newContext({ baseURL: DEMO_URL });
  const page = await context.newPage();

  await page.goto("/setup");
  await page.fill("#code", readSetupCode("app-fresh-demo.log"));
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL(/\/setup\/admin$/);
  await page.fill("#username", "demo.admin");
  await page.fill("#password", ADMIN_PASSWORD);
  await page.fill("#password_confirm", ADMIN_PASSWORD);
  await page.click('button[type="submit"]');
  await expect(page).toHaveURL(/\/setup\/club$/);

  await page.click('button:has-text("Explore with demo data")');
  await expect(page).toHaveURL(/\/admin\/league\/dashboard$/);
  await expect(page.locator("#demo-banner")).toContainText("Demo data");
  // The banner shows on every page.
  await page.goto("/admin/league/players");
  await expect(page.locator("#demo-banner")).toBeVisible();

  expect(parseInt(sqlite("SELECT COUNT(*) FROM clubs;", DEMO_DB)[0], 10)).toBeGreaterThan(0);

  await page.click('#demo-banner a:has-text("Clear demo data")');
  await expect(page).toHaveURL(/\/setup\/demo\/clear$/);
  await page.click('button:has-text("Clear demo data")');
  await expect(page).toHaveURL(/\/setup\/club$/);
  await expect(page.locator('[data-setup-step="club"]')).toBeVisible();
  await expect(page.locator("#demo-banner")).toHaveCount(0);

  // The administrator login survives; everything else is gone.
  expect(sqlite("SELECT username FROM users;", DEMO_DB)).toEqual(["demo.admin"]);
  expect(sqlite("SELECT COUNT(*) FROM clubs;", DEMO_DB)).toEqual(["0"]);
  expect(sqlite("SELECT COUNT(*) FROM fixtures;", DEMO_DB)).toEqual(["0"]);
  await expect(page.locator('input[name="club"]').first()).toBeVisible();
  await context.close();
});
