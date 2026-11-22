import { test, expect } from "@playwright/test";
import { continueAfterPasswordSignIn, loginAsCaptain, loginWith } from "./helpers/auth";

// Roles: see docs/roles.md. testcaptain is seeded (fixtures/seed.sql).

const CAPTAIN_PAGES = [
  "/admin/league/players",
  "/admin/league/teams",
  "/admin/league/fixtures",
  "/admin/league/planning",
  "/admin/league/points-table",
];

const ADMIN_PAGES = [
  "/admin/league/users",
  "/admin/league/sessions",
  "/admin/league/settings",
  "/admin/league/seasons",
  "/admin/league/audit-log",
];

test.describe("Captain role", () => {
  test("the dashboard shows captain navigation and no administrator links", async ({ page }) => {
    await loginAsCaptain(page);
    await expect(page.locator('[data-testid="dashboard-title"]')).toHaveText("Captain Dashboard");
    await expect(page.locator('[data-testid="user-role"]')).toHaveAttribute("data-role", "captain");
    for (const id of ["nav-players", "nav-planning"]) {
      await expect(page.locator(`[data-testid="${id}"]`)).toBeVisible();
    }
    for (const id of ["nav-users", "nav-sessions", "nav-settings", "nav-seasons", "audit-log-link"]) {
      await expect(page.locator(`[data-testid="${id}"]`)).toHaveCount(0);
    }
  });

  test("a captain reaches the captain pages", async ({ page }) => {
    await loginAsCaptain(page);
    for (const path of CAPTAIN_PAGES) {
      const response = await page.goto(path);
      expect(response?.status(), path).toBe(200);
      await expect(page.locator('[data-testid="forbidden-page"]'), path).toHaveCount(0);
    }
  });

  test("a captain is refused the administrator pages with the 403 page", async ({ page }) => {
    await loginAsCaptain(page);
    for (const path of ADMIN_PAGES) {
      const response = await page.goto(path);
      expect(response?.status(), path).toBe(403);
      await expect(page.locator('[data-testid="forbidden-page"]'), path).toBeVisible();
    }
  });
});

test.describe("Administrator role", () => {
  test("an administrator sees the administrator navigation", async ({ browser }) => {
    const context = await browser.newContext({ storageState: "auth-state.json" });
    const page = await context.newPage();
    await page.goto("/admin/league/dashboard");
    await expect(page.locator('[data-testid="dashboard-title"]')).toHaveText("Admin Dashboard");
    for (const id of ["nav-users", "nav-sessions", "nav-settings", "nav-seasons", "audit-log-link"]) {
      await expect(page.locator(`[data-testid="${id}"]`)).toBeVisible();
    }
    await context.close();
  });
});

// WI-153: testtournaments is seeded as a Tournaments only account with access
// to its own tournaments (fixtures/seed.sql).
test.describe("Tournaments only account", () => {
  test("signs in to the tournament home page and is refused the league", async ({ page }) => {
    await loginWith(page, "testtournaments", "testpassword123");
    await continueAfterPasswordSignIn(page);
    await page.waitForURL("**/tournament-home");
    await expect(page.locator('[data-testid="tournament-home"]')).toBeVisible();

    // One thing to do: open tournament management, or (with Tournaments
    // switched off for the club) a plain explanation.
    const open = page.locator('[data-testid="tournament-management-link"]');
    const off = page.locator('[data-testid="tournaments-switched-off"]');
    expect((await open.count()) + (await off.count())).toBe(1);
    if (await open.count()) {
      await expect(open).toHaveAttribute("href", "/tournaments/");
    }
    await expect(page.locator('[data-testid="tournament-home-league-link"]')).toHaveCount(0);

    for (const path of ["/admin/league/dashboard", "/admin/league/players", "/admin/league/users"]) {
      const response = await page.goto(path);
      expect(response?.status(), path).toBe(403);
      await expect(page.locator('[data-testid="forbidden-page"]'), path).toBeVisible();
    }
    await page.locator('[data-testid="forbidden-tournament-home-link"]').click();
    await page.waitForURL("**/tournament-home");
  });

  test("a captain without tournament access is not offered the tournament home", async ({ page }) => {
    await loginAsCaptain(page);
    await expect(page.locator('[data-testid="nav-tournament-home"]')).toHaveCount(0);
    const response = await page.goto("/tournament-home");
    expect(response?.status()).toBe(403);
    await expect(page.locator('[data-testid="tournament-home-refused"]')).toBeVisible();
  });
});
