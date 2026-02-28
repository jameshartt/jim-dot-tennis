import { test, expect } from "../fixtures/test-fixtures";
import {
  expectNoErrorBanner,
  expectTitleContains,
} from "../helpers/assertions";

test.describe("Admin Points Table", () => {
  test("points table page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/points-table");
    expect(response?.status()).toBe(200);
    await expectTitleContains(adminPage, "Jim.Tennis");
    await expectNoErrorBanner(adminPage);
  });

  test("points table heading is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/points-table");
    await expect(adminPage.locator("h1")).toContainText("Points Table");
  });

  test("points info section is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/points-table");
    const pointsInfo = adminPage.locator(".points-info");
    await expect(pointsInfo).toBeVisible();
  });

  test("men and women sections are present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/points-table");
    // Ensure we're on the points table page, not redirected to login
    await expect(adminPage).not.toHaveURL(/\/login/);
    const sections = adminPage.locator(".points-section");
    await expect(sections).toHaveCount(2);
  });

  test("points tables container is rendered", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/points-table");
    await expect(adminPage).not.toHaveURL(/\/login/);
    const tablesContainer = adminPage.locator(".points-tables");
    await expect(tablesContainer).toBeAttached();
  });

  test("week info is displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/points-table");
    await expect(adminPage).not.toHaveURL(/\/login/);
    const weekInfo = adminPage.locator(".week-info");
    await expect(weekInfo).toBeVisible();
  });
});
