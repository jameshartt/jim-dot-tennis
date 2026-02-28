import { test, expect } from "@playwright/test";
import { loginAsAdmin } from "./helpers/auth";

const VALID_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";

/**
 * Check that the page has no horizontal overflow (content wider than viewport).
 */
async function expectNoHorizontalOverflow(
  page: import("@playwright/test").Page,
) {
  const overflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth,
  );
  expect(overflow, "Page has horizontal overflow").toBe(false);
}

test.describe("Responsive - Mobile (375x812)", () => {
  test.use({ viewport: { width: 375, height: 812 } });

  test("login page renders on mobile", async ({ page }) => {
    await page.goto("/login");
    await expect(page.locator("h1, h2").first()).toBeVisible();
    await expectNoHorizontalOverflow(page);
  });

  test("dashboard renders on mobile", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/admin/league/dashboard");
    await expect(page.locator("h1")).toContainText("Admin Dashboard");
    await expectNoHorizontalOverflow(page);
  });

  test("availability renders on mobile", async ({ page }) => {
    await page.goto(`/my-availability/${VALID_TOKEN}`);
    const container = page.locator(".availability-container");
    await expect(container).toBeVisible();
    await expectNoHorizontalOverflow(page);
  });

  test("standings renders on mobile", async ({ page }) => {
    await page.goto("/standings");
    await expect(page.locator("h1")).toContainText("League Standings");
    const tabs = page.locator(".division-tab");
    const count = await tabs.count();
    expect(count).toBeGreaterThanOrEqual(1);
    await expectNoHorizontalOverflow(page);
  });

  test("fixtures list renders on mobile", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/admin/league/fixtures");
    await expect(page.locator("h1")).toContainText("Fixture Management");
    await expectNoHorizontalOverflow(page);
  });
});

test.describe("Responsive - Tablet (768x1024)", () => {
  test.use({ viewport: { width: 768, height: 1024 } });

  test("dashboard renders on tablet", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/admin/league/dashboard");
    await expect(page.locator("h1")).toContainText("Admin Dashboard");
    await expectNoHorizontalOverflow(page);
  });

  test("fixtures page renders on tablet", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/admin/league/fixtures");
    await expect(page.locator("h1")).toContainText("Fixture Management");
    await expectNoHorizontalOverflow(page);
  });

  test("standings renders on tablet", async ({ page }) => {
    await page.goto("/standings");
    await expect(page.locator("h1")).toContainText("League Standings");
    await expectNoHorizontalOverflow(page);
  });

  test("selection overview renders on tablet", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/admin/league/selection-overview");
    await expect(page.locator("h1")).toContainText(
      "Captain Selection Overview",
    );
    await expectNoHorizontalOverflow(page);
  });
});
