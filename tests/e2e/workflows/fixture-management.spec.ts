import { test, expect } from "../fixtures/test-fixtures";

test.describe("Fixture Detail & Editing", () => {
  test("detail page shows fixture information heading", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/fixtures/1");
    await expect(adminPage.locator("h2")).toContainText("Fixture Information");
  });

  test("detail page has results and edit links", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1");
    const resultsLink = adminPage.locator('a[href*="/results"]');
    await expect(resultsLink).toBeVisible();
    const editLink = adminPage.locator('a[href*="/edit"]');
    await expect(editLink).toBeVisible();
  });

  test("edit page loads", async ({ adminPage }) => {
    const response = await adminPage.goto(
      "/admin/league/fixtures/1/edit",
    );
    expect(response?.status()).toBe(200);
    await expect(adminPage.locator("h2")).toContainText("Edit Fixture");
  });

  test("edit form fields are present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/edit");
    await expect(adminPage.locator("#scheduled_date")).toBeVisible();
    await expect(adminPage.locator("#rescheduled_reason")).toBeVisible();
    await expect(adminPage.locator("#notes")).toBeVisible();
  });

  test("edit form saves changes", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/edit");

    // Fill in new date, reason, and notes
    await adminPage.fill("#scheduled_date", "2025-04-21T18:00");
    await adminPage.selectOption("#rescheduled_reason", "Weather");
    await adminPage.fill("#notes", "Rescheduled due to rain");

    // Submit the form
    await adminPage.click(".btn-primary");

    // Should redirect to fixture detail or show success
    await adminPage.waitForURL("**/admin/league/fixtures/1**");
    const pageContent = await adminPage.textContent("body");
    // Either redirected to detail or shows success message
    expect(
      pageContent?.includes("Fixture Information") ||
        pageContent?.includes("success") ||
        pageContent?.includes("Success"),
    ).toBe(true);
  });
});

test.describe("Captain Selection Overview", () => {
  test("page loads with heading", async ({ adminPage }) => {
    const response = await adminPage.goto(
      "/admin/league/selection-overview",
    );
    expect(response?.status()).toBe(200);
    await expect(adminPage.locator("h1")).toContainText(
      "Captain Selection Overview",
    );
  });

  test("week dropdown is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/selection-overview");
    const weekDropdown = adminPage.locator(".week-dropdown");
    await expect(weekDropdown).toBeVisible();
  });

  test("division filter chips are present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/selection-overview");
    const filterChips = adminPage.locator(".division-filter-chip");
    const count = await filterChips.count();
    expect(count).toBeGreaterThanOrEqual(2);
  });

  test("fixture cards are displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/selection-overview");
    const fixtureCards = adminPage.locator(".fixture-card");
    const count = await fixtureCards.count();
    expect(count).toBeGreaterThanOrEqual(1);
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("St Ann");
  });

  test("team selection link is available", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/selection-overview");
    const teamSelectionLink = adminPage.locator(
      'a[href*="/team-selection"]',
    );
    const count = await teamSelectionLink.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });
});
