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
    const addBtn = adminPage.locator('a.btn-add[href="/admin/league/players/new"]');
    await expect(addBtn).toBeVisible();
  });
});

// Regression for the nested-form bug introduced with the captain notes section
// (Sprint 017 WI-105). Captain notes contained their own <form> elements nested
// inside the player edit form, which is invalid HTML. The browser parser
// honours inner </form> closing tags, prematurely closing the outer form and
// orphaning the "Update Player" button — so clicking it submitted nothing.
test.describe("Admin Players — edit form submission", () => {
  test("Update Player button is a descendant of the edit form", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/players/p-alice/edit");
    const updateButton = adminPage.locator(
      'form.edit-form button.btn-primary:has-text("Update Player")',
    );
    await expect(updateButton).toBeVisible();
  });

  test("clicking Update Player submits the form and redirects to the players list", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/players/p-alice/edit");
    await adminPage
      .locator('button.btn-primary:has-text("Update Player")')
      .click();
    await expect(adminPage).toHaveURL(/\/admin\/league\/players\/?$/);
  });
});
