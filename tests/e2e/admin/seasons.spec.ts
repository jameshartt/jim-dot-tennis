import { test, expect } from "../fixtures/test-fixtures";
import {
  expectNoErrorBanner,
  expectTitleContains,
} from "../helpers/assertions";

test.describe("Admin Seasons", () => {
  test("seasons list page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/seasons");
    expect(response?.status()).toBe(200);
    await expectTitleContains(adminPage, "Jim.Tennis");
    await expectNoErrorBanner(adminPage);
  });

  test("seasons heading is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/seasons");
    // Verify we're authenticated (not redirected to login)
    await expect(adminPage).not.toHaveURL(/\/login/);
    await expect(adminPage.locator("h1")).toContainText("Season Management");
  });

  test("seeded season is displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/seasons");
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("Summer 2025");
  });

  test("active season has active badge", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/seasons");
    const activeBadge = adminPage.locator(".active-badge");
    await expect(activeBadge).toBeVisible();
    await expect(activeBadge).toContainText("ACTIVE");
  });

  test("seasons table has expected rows", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/seasons");
    const seasonsTable = adminPage.locator(".seasons-table");
    await expect(seasonsTable).toBeVisible();
    const rows = seasonsTable.locator("tbody tr");
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test("season setup page loads", async ({ adminPage }) => {
    const response = await adminPage.goto(
      "/admin/league/seasons/setup?id=1",
    );
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });
});
