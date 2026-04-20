// Coverage for the team-first planning matrix: a team super-header over
// fixture columns, each fixture header linking to its team-selection page,
// and cells acting as in-place add/remove toggles for fixture_players.

import { test, expect } from "../fixtures/test-fixtures";

test.describe("Planning matrix — team-first columns + cell toggle", () => {
  // Toggle tests share the same (player, fixture, team) row in fixture_players,
  // so they must run sequentially — parallel workers would race on the DB.
  test.describe.configure({ mode: "serial" });

  test("super-header groups columns by St Ann's team", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/planning?week=1");
    // Seed has two St Ann's home fixtures in week 1 (team 1 & team 2).
    await expect(
      adminPage.locator('[data-testid="team-group-1"]'),
    ).toContainText("St Ann");
    await expect(
      adminPage.locator('[data-testid="team-group-2"]'),
    ).toContainText("St Ann");
    // The super-header row is one node above the regular header.
    await expect(
      adminPage.locator('[data-testid="matrix-team-row"]'),
    ).toBeVisible();
  });

  test("fixture column header links to team-selection page", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/planning?week=1");
    const link = adminPage.locator('[data-testid="fixture-link-1-1"]');
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute(
      "href",
      "/admin/league/fixtures/1/team-selection",
    );
  });

  test("clicking a matrix cell toggles the player in/out of team selection", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/planning?week=1");
    const cell = adminPage.locator('[data-testid="cell-p-alice-1-1"]');
    await expect(cell).toBeVisible();
    // Baseline: player is not in the seeded selection pool.
    await expect(cell).toHaveAttribute("data-in-selection", "false");

    await cell.click();
    // After one toggle the cell should report itself as in-selection. HTMX
    // swaps the TD out, so re-query rather than reuse the stale handle.
    const after = adminPage.locator('[data-testid="cell-p-alice-1-1"]');
    await expect(after).toHaveAttribute("data-in-selection", "true");

    // Clicking again removes.
    await after.click();
    const final = adminPage.locator('[data-testid="cell-p-alice-1-1"]');
    await expect(final).toHaveAttribute("data-in-selection", "false");
  });

  test("cell selection state survives a full matrix re-render", async ({
    adminPage,
  }) => {
    // Add Alice via the cell toggle, reload the page, assert the cell still
    // reports in-selection — proves the prefetch picks the change up.
    await adminPage.goto("/admin/league/planning?week=1");
    const cell = adminPage.locator('[data-testid="cell-p-alice-1-1"]');
    await cell.click();
    await expect(
      adminPage.locator('[data-testid="cell-p-alice-1-1"]'),
    ).toHaveAttribute("data-in-selection", "true");

    await adminPage.goto("/admin/league/planning?week=1");
    await expect(
      adminPage.locator('[data-testid="cell-p-alice-1-1"]'),
    ).toHaveAttribute("data-in-selection", "true");

    // Clean up so the spec stays idempotent for re-runs on a reused DB.
    await adminPage.locator('[data-testid="cell-p-alice-1-1"]').click();
    await expect(
      adminPage.locator('[data-testid="cell-p-alice-1-1"]'),
    ).toHaveAttribute("data-in-selection", "false");
  });
});
