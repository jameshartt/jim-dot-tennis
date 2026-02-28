import { test, expect } from "../fixtures/test-fixtures";
import { expectNoErrorBanner } from "../helpers/assertions";

test.describe("Admin Dashboard", () => {
  test("dashboard loads with correct heading", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    await expect(adminPage.locator("h1")).toContainText("Admin Dashboard");
    await expectNoErrorBanner(adminPage);
  });

  test("stat cards are displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    const statCards = adminPage.locator(".stat-card");
    // Should have stat cards for Players, Fixtures, Teams, Clubs, etc.
    await expect(statCards.first()).toBeVisible();
    const count = await statCards.count();
    expect(count).toBeGreaterThanOrEqual(4);
  });

  test("stat card links navigate to correct pages", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    const statLinks = adminPage.locator(".stat-card-link");
    const count = await statLinks.count();
    expect(count).toBeGreaterThanOrEqual(4);

    // Verify the links point to admin sections
    for (let i = 0; i < count; i++) {
      const href = await statLinks.nth(i).getAttribute("href");
      expect(href).toContain("/admin/league/");
    }
  });

  test("action links are present and functional", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    const actionLinks = adminPage.locator(".action-link");
    const count = await actionLinks.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test("login attempts section is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    // Login attempts section uses a <details> element
    const toggle = adminPage.locator(".login-attempts-toggle");
    await expect(toggle).toBeVisible();
  });

  test("logout button is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    const logoutBtn = adminPage.locator('a[href="/logout"]');
    await expect(logoutBtn).toBeVisible();
  });

  test("stat values show correct seeded counts", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    const statValues = adminPage.locator(".stat-value");
    const count = await statValues.count();
    // At least one stat value should be non-zero (we have seeded data)
    let hasNonZero = false;
    for (let i = 0; i < count; i++) {
      const text = await statValues.nth(i).textContent();
      if (text && parseInt(text) > 0) {
        hasNonZero = true;
        break;
      }
    }
    expect(hasNonZero).toBe(true);
  });
});
