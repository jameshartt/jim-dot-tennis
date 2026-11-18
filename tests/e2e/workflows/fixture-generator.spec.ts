import { test, expect } from "@playwright/test";
import fs from "fs";

// The fixture generator is public and writes nothing to the database, so
// these tests need no login and leave the seeded data untouched. It starts
// from the live league tables, or the bundled copy when the league website
// cannot be reached; both are the 2026 final tables.

test.describe("Fixture generator", () => {
  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto("/fixture-generator");
    await expect(page.locator(".division")).toHaveCount(4);
  });

  test("is reachable without logging in", async ({ page }) => {
    await expect(page).toHaveURL(/\/fixture-generator$/);
    await expect(page.locator("h1")).toHaveText("Fixture Generator");
    await expect(page.locator(".memorial")).toContainText("Mick Cox");
  });

  test("applies promotion, relegation and the two-teams-per-club rule", async ({
    page,
  }) => {
    await page.getByRole("button", { name: /Apply promotion/ }).click();

    const divisions = page.locator(".division");
    await expect(divisions.nth(0).locator("[data-team]")).toHaveCount(10);
    await expect(divisions.nth(3).locator("[data-team]")).toHaveCount(12);

    // Dyke C finished second in Division 3 but Dyke already has two teams in Division 2.
    await expect(divisions.nth(2).locator('[data-team="Dyke C"]')).toBeVisible();
    await expect(divisions.nth(1).locator('[data-team="Preston Park B"]')).toBeVisible();
    await expect(page.locator(".ruling")).toContainText(["Dyke C can't go up"]);

    // No division ends up with three teams from one club.
    await expect(page.locator(".banner.bad")).toHaveCount(0);
  });

  test("a committee override that breaks the club cap blocks generation", async ({
    page,
  }) => {
    await page.getByRole("button", { name: /Apply promotion/ }).click();
    await page.locator('[data-team="Hove E"] .team-btn').click();
    await page.locator(".sheet .choice", { hasText: "Division 3" }).click();

    await expect(page.locator(".banner.bad")).toContainText("Hove");
    await expect(page.getByRole("button", { name: /Generate fixtures/ })).toBeDisabled();

    await page.locator(".ruling", { hasText: "Hove E moved" }).getByRole("button", { name: "Undo" }).click();
    await expect(page.locator(".banner.bad")).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Generate fixtures/ })).toBeEnabled();
  });

  test("generates, accepts, downloads and reloads a fixture file", async ({
    page,
  }) => {
    await page.getByRole("button", { name: /Apply promotion/ }).click();
    await page.getByRole("button", { name: /Generate fixtures/ }).click();

    await expect(page.locator(".intro h2")).toHaveText("402 fixtures, drawn.", { timeout: 30_000 });
    // Division 1: nine week-pairs of five matches.
    await expect(page.locator(".cards .card")).toHaveCount(9);
    await expect(page.locator(".cards .card").first().locator(".fx")).toHaveCount(5);

    // Division 4 has 12 teams, so two extra rounds per half.
    await page.getByRole("tab", { name: /Division 4/ }).click();
    await expect(page.locator(".cards .card")).toHaveCount(11);
    await expect(page.locator(".cards .card.extra")).toHaveCount(2);

    await page.getByRole("button", { name: /Accept these fixtures/ }).click();
    await expect(page.locator(".stamp")).toHaveText("Accepted");

    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.getByRole("button", { name: "Download fixture file" }).click(),
    ]);
    expect(download.suggestedFilename()).toBe("bhplta-2027-fixtures.jimtennis.json");
    const path = await download.path();
    const file = JSON.parse(fs.readFileSync(path!, "utf8"));
    expect(file.format).toBe("jim.tennis/fixtures");
    expect(file.status).toBe("accepted");
    expect(file.season.year).toBe(2027);
    expect(file.divisions.map((d: { fixtures: unknown[] }) => d.fixtures.length)).toEqual([90, 90, 90, 132]);

    // Load the same file back, as another club would.
    await page.locator("#load-file-input").setInputFiles(path!);
    await expect(page.locator(".intro h2")).toHaveText("2027 fixtures, accepted");
    await expect(page.locator(".intro .pill")).toHaveText("Unaltered");
  });

  test("rejects a file that is not a fixture file", async ({ page }) => {
    await page.locator("#load-file-input").setInputFiles({
      name: "holiday.json",
      mimeType: "application/json",
      buffer: Buffer.from('{"hello":"world"}'),
    });
    await expect(page.locator("#toast")).toContainText("isn't a jim.tennis fixture file");
    await expect(page.locator(".division")).toHaveCount(4);
  });

  test("works on a phone", async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await page.getByRole("button", { name: /Apply promotion/ }).click();
    await page.getByRole("button", { name: /Generate fixtures/ }).click();
    await expect(page.locator(".cards .card").first()).toBeVisible({ timeout: 30_000 });
    const overflow = await page.evaluate(
      () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
    );
    expect(overflow).toBeLessThanOrEqual(1);
  });
});
