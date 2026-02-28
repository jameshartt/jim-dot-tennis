import { test, expect } from "../fixtures/test-fixtures";
import {
  expectNoErrorBanner,
  expectTitleContains,
} from "../helpers/assertions";

test.describe("Admin Clubs", () => {
  test("clubs list page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/clubs");
    expect(response?.status()).toBe(200);
    await expectTitleContains(adminPage, "Jim.Tennis");
    await expectNoErrorBanner(adminPage);
  });

  test("clubs heading is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/clubs");
    await expect(adminPage.locator("h1")).toContainText("Club Management");
  });

  test("seeded clubs are displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/clubs");
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("St Ann's Tennis Club");
    expect(pageContent).toContain("Hove Park Tennis Club");
  });

  test("clubs table is present with rows", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/clubs");
    const clubsTable = adminPage.locator(".clubs-table");
    await expect(clubsTable).toBeVisible();
    const rows = clubsTable.locator("tbody tr");
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(2); // 2 seeded clubs
  });

  test("club detail page loads for seeded club", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/clubs/1");
    const response = await adminPage.goto("/admin/league/clubs/1");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("St Ann");
  });

  test("club count is shown", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/clubs");
    const clubCount = adminPage.locator(".club-count");
    await expect(clubCount).toBeVisible();
  });
});
