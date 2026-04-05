import { test, expect } from "../fixtures/test-fixtures";
import {
  expectNoErrorBanner,
  expectTitleContains,
} from "../helpers/assertions";
import { TEST_CONFIG } from "../fixtures/test-config";

test.describe("Admin Teams", () => {
  test("teams list page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/teams");
    expect(response?.status()).toBe(200);
    await expectTitleContains(adminPage, "Jim.Tennis");
    await expectNoErrorBanner(adminPage);
  });

  test("teams heading is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/teams");
    await expect(adminPage.locator("h1")).toContainText("Team Management");
  });

  test("seeded teams are displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/teams");
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain(TEST_CONFIG.HOME_TEAM_A);
    expect(pageContent).toContain(TEST_CONFIG.HOME_TEAM_B);
  });

  test("teams table has expected rows", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/teams");
    const teamsTable = adminPage.locator(".teams-table").first();
    await expect(teamsTable).toBeVisible();
    const rows = teamsTable.locator("tbody tr");
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(2); // At least our club's teams
  });

  test("away teams page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/teams/away");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
    const pageContent = await adminPage.textContent("body");
    // Should show opponent teams
    expect(pageContent).toContain("Hove Park");
  });
});
