import { test, expect } from "../fixtures/test-fixtures";

test.describe("Match Results - Page Structure", () => {
  test("results page loads for fixture 2", async ({ adminPage }) => {
    const response = await adminPage.goto(
      "/admin/league/fixtures/2/results",
    );
    expect(response?.status()).toBe(200);
    const headerCard = adminPage.locator(".fixture-header-card");
    await expect(headerCard).toBeVisible();
  });

  test("four matchup cards are rendered", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/2/results");
    await adminPage.waitForLoadState("networkidle");
    const matchupCards = adminPage.locator(".matchup-card");
    await expect(matchupCards.first()).toBeVisible();
    await expect(matchupCards).toHaveCount(4);
  });

  test("score inputs are present for all matchups", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/2/results");
    await adminPage.waitForLoadState("networkidle");
    const scoreInputs = adminPage.locator(".score-input");
    const count = await scoreInputs.count();
    // 4 matchups × 4 visible inputs (home_set1, away_set1, home_set2, away_set2) = 16 minimum
    expect(count).toBeGreaterThanOrEqual(16);
  });

  test("conceded checkboxes are present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/2/results");
    await adminPage.waitForLoadState("networkidle");
    const concededCheckboxes = adminPage.locator(".conceded-checkbox");
    await expect(concededCheckboxes).toHaveCount(4);
  });

  test("set 3 toggle is present for each matchup", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/2/results");
    await adminPage.waitForLoadState("networkidle");
    const set3Toggles = adminPage.locator(".set3-checkbox");
    await expect(set3Toggles).toHaveCount(4);
  });
});

test.describe("Match Results - Validation", () => {
  test("invalid scores show error messages on fixture 1", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/fixtures/1/results");
    await adminPage.waitForLoadState("networkidle");

    // Enter invalid score (5-5 is not a valid tennis set score)
    await adminPage.fill(
      'input[name="matchup_1_home_set1"]',
      "5",
    );
    await adminPage.fill(
      'input[name="matchup_1_away_set1"]',
      "5",
    );
    await adminPage.fill(
      'input[name="matchup_1_home_set2"]',
      "6",
    );
    await adminPage.fill(
      'input[name="matchup_1_away_set2"]',
      "4",
    );

    // Submit
    await adminPage.click(".btn-submit");
    await adminPage.waitForLoadState("networkidle");

    // Should stay on the page and show validation errors
    const errorMsg = adminPage.locator(".error-msg").first();
    await expect(errorMsg).toBeVisible();
  });
});

test.describe.serial("Match Results - Submission", () => {
  test("submit valid scores completes fixture 2", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/2/results");
    await adminPage.waitForLoadState("networkidle");

    // Verify matchup cards are present before filling
    await expect(adminPage.locator(".matchup-card").first()).toBeVisible();

    // Fill in valid scores for all 4 matchups (IDs 5-8)
    for (const matchupId of [5, 6, 7, 8]) {
      await adminPage.fill(
        `input[name="matchup_${matchupId}_home_set1"]`,
        "6",
      );
      await adminPage.fill(
        `input[name="matchup_${matchupId}_away_set1"]`,
        "4",
      );
      await adminPage.fill(
        `input[name="matchup_${matchupId}_home_set2"]`,
        "6",
      );
      await adminPage.fill(
        `input[name="matchup_${matchupId}_away_set2"]`,
        "3",
      );
    }

    // Submit the form
    await adminPage.click(".btn-submit");

    // Should redirect to fixture detail with Completed status
    await adminPage.waitForURL("**/admin/league/fixtures/2");
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("Completed");
  });
});
