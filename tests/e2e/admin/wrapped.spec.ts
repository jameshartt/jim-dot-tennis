import { test, expect } from "../fixtures/test-fixtures";
import { test as baseTest } from "@playwright/test";
import { expectNoErrorBanner } from "../helpers/assertions";

test.describe("Admin Wrapped", () => {
  test("admin wrapped page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/wrapped");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });

  test("wrapped container is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/wrapped");
    const container = adminPage.locator(".wrapped-container");
    await expect(container).toBeVisible();
  });
});

baseTest.describe("Public Club Wrapped", () => {
  baseTest("club wrapped page responds", async ({ page }) => {
    const response = await page.goto("/club/wrapped");
    // May require password — just verify the page returns a response
    const status = response?.status();
    expect(status).toBeDefined();
    expect(status).toBeLessThan(500); // No server errors
  });
});
