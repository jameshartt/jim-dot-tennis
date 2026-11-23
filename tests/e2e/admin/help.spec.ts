import { test, expect } from "../fixtures/test-fixtures";
import { test as plain } from "@playwright/test";
import { loginAsCaptain } from "../helpers/auth";

// WI-149: every main admin page has a help link that opens the matching
// section of the guides the app serves itself (internal/help).

const pages = [
  { name: "Dashboard", url: "/admin/league/dashboard" },
  { name: "Players", url: "/admin/league/players" },
  { name: "Fixtures", url: "/admin/league/fixtures" },
  { name: "Teams", url: "/admin/league/teams" },
  { name: "Clubs", url: "/admin/league/clubs" },
  { name: "Seasons", url: "/admin/league/seasons" },
  { name: "Season setup", url: "/admin/league/seasons/setup?id=1" },
  { name: "Users", url: "/admin/league/users" },
  { name: "Sessions", url: "/admin/league/sessions" },
  { name: "Audit log", url: "/admin/league/audit-log" },
  { name: "Backups", url: "/admin/league/backups" },
  { name: "Club settings", url: "/admin/league/settings" },
  { name: "Club data import", url: "/admin/league/club-data-import" },
  { name: "Match card import", url: "/admin/league/match-card-import" },
  { name: "Planning", url: "/admin/league/planning" },
  { name: "Selection overview", url: "/admin/league/selection-overview" },
  { name: "Points table", url: "/admin/league/points-table" },
  { name: "Preferred names", url: "/admin/league/preferred-names" },
  { name: "Tournaments", url: "/admin/league/tournaments" },
];

test.describe("Help links on the admin pages", () => {
  for (const p of pages) {
    test(`${p.name} has a help link that opens its guide section`, async ({ adminPage }) => {
      const response = await adminPage.goto(p.url);
      expect(response?.status()).toBe(200);

      const link = adminPage.locator('[data-testid="help-link"]');
      await expect(link).toHaveCount(1);
      const href = await link.getAttribute("href");
      expect(href).toMatch(/^\/help\/(admin|captain|tournaments)(#[a-z0-9-]+)?$/);

      await link.click();
      await expect(adminPage.locator('[data-testid="help-guide"]')).toBeVisible();
      const hash = new URL(adminPage.url()).hash;
      if (hash) {
        // The section the link names is on the page.
        await expect(adminPage.locator(hash)).toHaveCount(1);
      }
    });
  }

  test("the contents page lists the guides", async ({ adminPage }) => {
    const response = await adminPage.goto("/help");
    expect(response?.status()).toBe(200);
    for (const slug of ["admin", "captain", "player", "tournaments"]) {
      await expect(adminPage.locator(`[data-testid="help-guide-${slug}"]`)).toBeVisible();
    }
  });

  test("the roles table is in the administrator guide", async ({ adminPage }) => {
    await adminPage.goto("/help/admin#what-administrators-and-captains-can-do");
    await expect(adminPage.locator("article table").last()).toContainText("Seasons: create, copy");
  });

  test("a guide fits a phone screen", async ({ adminPage }) => {
    await adminPage.setViewportSize({ width: 375, height: 812 });
    await adminPage.goto("/help/captain");
    const overflow = await adminPage.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(1);
  });

  test("a captain can open the captain guide from the dashboard", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    await loginAsCaptain(page);
    await page.locator('[data-testid="help-link"]').click();
    await expect(page).toHaveURL(/\/help\/captain$/);
    await expect(page.locator('[data-testid="help-guide"]')).toBeVisible();
    await context.close();
  });
});

plain.describe("Guides when signed out", () => {
  plain("the player sheet is public", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    const response = await page.goto("/help/player");
    expect(response?.status()).toBe(200);
    await expect(page.locator("article h1")).toHaveText("Your link");
    await context.close();
  });

  plain("the other guides ask for a sign-in", async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    await page.goto("/help/admin");
    await expect(page).toHaveURL(/\/login/);
    await context.close();
  });
});
