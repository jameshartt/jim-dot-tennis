import { test, expect } from "../fixtures/test-fixtures";
import {
  expectNoErrorBanner,
  expectTitleContains,
} from "../helpers/assertions";

test.describe("Admin Players", () => {
  test("players list page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/players");
    expect(response?.status()).toBe(200);
    await expectTitleContains(adminPage, "Jim.Tennis");
    await expectNoErrorBanner(adminPage);
  });

  test("players heading is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/players");
    await expect(adminPage.locator("h2")).toContainText("All Players");
  });

  test("seeded players are displayed in the table", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/players");
    const playersTable = adminPage.locator(".players-table");
    await expect(playersTable).toBeVisible();

    // Check for seeded player names
    const tableBody = adminPage.locator("#players-tbody");
    await expect(tableBody).toBeVisible();
    const rows = tableBody.locator("tr");
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(8); // 8 seeded players
  });

  test("search input is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/players");
    const searchInput = adminPage.locator("#search");
    await expect(searchInput).toBeVisible();
  });

  test("player row contains expected data", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/players");
    // Look for one of our seeded players
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("Alice");
    expect(pageContent).toContain("Smith");
  });

  test("add player button is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/players");
    const addBtn = adminPage.locator(".btn-add");
    await expect(addBtn).toBeVisible();
  });
});
