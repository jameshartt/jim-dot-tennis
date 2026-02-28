import { test, expect } from "../fixtures/test-fixtures";
import {
  expectNoErrorBanner,
  expectTitleContains,
} from "../helpers/assertions";

test.describe("Admin Fixtures", () => {
  test("fixtures list page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/fixtures");
    expect(response?.status()).toBe(200);
    await expectTitleContains(adminPage, "Jim.Tennis");
    await expectNoErrorBanner(adminPage);
  });

  test("fixtures heading is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures");
    await expect(adminPage.locator("h1")).toContainText("Fixture Management");
  });

  test("seeded fixtures are displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures");
    const pageContent = await adminPage.textContent("body");
    // Seeded fixtures have St Ann's and Hove Park teams
    expect(pageContent).toContain("St Ann");
    expect(pageContent).toContain("Hove Park");
  });

  test("fixture detail page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/fixtures/1");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });

  test("fixture status badges are visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures");
    // Seeded fixtures have 'Scheduled' status
    const statusBadges = adminPage.locator(".status-badge");
    const count = await statusBadges.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test("week overview page loads", async ({ adminPage }) => {
    const response = await adminPage.goto(
      "/admin/league/fixtures/week-overview",
    );
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });

  test("division filter is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures");
    const filterSection = adminPage.locator(".filter-section");
    await expect(filterSection).toBeVisible();
  });
});
