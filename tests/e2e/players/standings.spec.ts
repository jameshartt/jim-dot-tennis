import { test, expect } from "@playwright/test";
import { expectNoErrorBanner } from "../helpers/assertions";

test.describe("League Standings", () => {
  test("standings page loads without authentication", async ({ page }) => {
    const response = await page.goto("/standings");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(page);
  });

  test("standings container is present", async ({ page }) => {
    await page.goto("/standings");
    const container = page.locator(".standings-container");
    await expect(container).toBeVisible();
  });

  test("standings heading is displayed", async ({ page }) => {
    await page.goto("/standings");
    await expect(page.locator("h1")).toContainText("League Standings");
  });

  test("division tabs are present", async ({ page }) => {
    await page.goto("/standings");
    const divisionTabs = page.locator(".division-tab");
    const count = await divisionTabs.count();
    expect(count).toBeGreaterThanOrEqual(1); // At least one division
  });

  test("standings table area is present", async ({ page }) => {
    await page.goto("/standings");
    const tableArea = page.locator("#standings-table");
    await expect(tableArea).toBeVisible();
  });

  test("standings show seeded divisions", async ({ page }) => {
    await page.goto("/standings");
    const pageContent = await page.textContent("body");
    expect(pageContent).toContain("Division 1");
  });

  test("controls bar with season select is present", async ({ page }) => {
    await page.goto("/standings");
    const controlsBar = page.locator(".controls-bar");
    await expect(controlsBar).toBeVisible();
  });
});
