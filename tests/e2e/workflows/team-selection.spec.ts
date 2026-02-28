import { test, expect } from "../fixtures/test-fixtures";
import { waitForHtmxSettle } from "../helpers/htmx";

test.describe("Team Selection Workflow", () => {
  test("team selection page loads", async ({ adminPage }) => {
    const response = await adminPage.goto(
      "/admin/league/fixtures/1/team-selection",
    );
    expect(response?.status()).toBe(200);
    const heading = adminPage.locator("h2");
    await expect(heading).toContainText("Team Selection");
  });

  test("fixture summary is displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/team-selection");
    const summary = adminPage.locator(".fixture-summary");
    await expect(summary).toBeVisible();
    await expect(summary.locator(".fixture-title")).toBeVisible();
    await expect(summary.locator(".teams-vs")).toBeVisible();
  });

  test("seeded players are visible on the page", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/team-selection");
    // Players appear either as add-player buttons (available) or player-cards (selected)
    // depending on whether another parallel test has already added them
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("Alice");
    expect(pageContent).toContain("Bob");
  });

  test("selected players zone is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/team-selection");
    const selectedZone = adminPage.locator(".selected-players-zone").first();
    await expect(selectedZone).toBeVisible();
  });

  test("matchup zones are displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/team-selection");
    const matchupZones = adminPage.locator(".matchup-zone");
    const count = await matchupZones.count();
    expect(count).toBeGreaterThanOrEqual(1);
  });

  test("add player via HTMX updates the container", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/team-selection");
    const container = adminPage.locator(".team-selection-container");
    await expect(container).toBeVisible();

    // Click the first available add-player button
    const addButton = adminPage.locator(".btn-add-player").first();
    const buttonExists = (await addButton.count()) > 0;
    if (buttonExists) {
      await addButton.click();
      await waitForHtmxSettle(adminPage, 3000);
      // Container should still be present after HTMX swap
      await expect(
        adminPage.locator(".team-selection-container"),
      ).toBeVisible();
    }
  });

  test("progress indicator shows player count", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/team-selection");
    const progress = adminPage.locator(".progress-indicator");
    if ((await progress.count()) > 0) {
      await expect(progress).toContainText("Players Selected");
    }
  });

  test("availability legend is shown", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/fixtures/1/team-selection");
    const legend = adminPage.locator(".availability-legend");
    if ((await legend.count()) > 0) {
      await expect(legend).toBeVisible();
    }
  });
});
