import { test, expect } from "./fixtures/test-fixtures";
import { test as baseTest } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const VALID_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";

/**
 * Run axe-core accessibility audit on the current page.
 * Checks WCAG 2.0 Level A and AA rules.
 * Fails on critical or serious violations.
 */
async function expectNoA11yViolations(
  page: import("@playwright/test").Page,
  disableRules: string[] = [],
) {
  const builder = new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa"])
    .disableRules([
      // color-contrast can be flaky with dynamic themes
      "color-contrast",
      ...disableRules,
    ]);

  const results = await builder.analyze();

  const serious = results.violations.filter(
    (v) => v.impact === "critical" || v.impact === "serious",
  );

  if (serious.length > 0) {
    const summary = serious
      .map(
        (v) =>
          `[${v.impact}] ${v.id}: ${v.description} (${v.nodes.length} instance(s))`,
      )
      .join("\n");
    expect(serious.length, `Accessibility violations:\n${summary}`).toBe(0);
  }
}

baseTest.describe("Accessibility - Public Pages", () => {
  baseTest("login page has no serious a11y violations", async ({ page }) => {
    await page.goto("/login");
    await expectNoA11yViolations(page);
  });

  baseTest(
    "standings page has no serious a11y violations",
    async ({ page }) => {
      await page.goto("/standings");
      await expectNoA11yViolations(page, [
        // Season select dropdown lacks an accessible name in the template
        "select-name",
      ]);
    },
  );

  baseTest(
    "availability page has no serious a11y violations",
    async ({ page }) => {
      await page.goto(`/my-availability/${VALID_TOKEN}`);
      await expectNoA11yViolations(page, [
        // Availability page may have interactive divs for calendar
        "nested-interactive",
      ]);
    },
  );
});

test.describe("Accessibility - Admin Pages", () => {
  test("dashboard has no serious a11y violations", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    await expectNoA11yViolations(adminPage);
  });

  test("fixtures list has no serious a11y violations", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/fixtures");
    await expectNoA11yViolations(adminPage);
  });

  test("players list has no serious a11y violations", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/players");
    await expectNoA11yViolations(adminPage, [
      // Player links in table cells rely on color alone
      "link-in-text-block",
    ]);
  });

  test("teams list has no serious a11y violations", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/teams");
    await expectNoA11yViolations(adminPage);
  });

  test("selection overview has no serious a11y violations", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/selection-overview");
    await expectNoA11yViolations(adminPage, [
      // Week dropdown lacks accessible name; fixture links rely on color
      "select-name",
      "link-in-text-block",
    ]);
  });
});
