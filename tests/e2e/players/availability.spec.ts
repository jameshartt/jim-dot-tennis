import { test, expect } from "@playwright/test";
import { expectNoErrorBanner } from "../helpers/assertions";

const VALID_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";

test.describe("Player Availability", () => {
  test("availability page loads with valid token", async ({ page }) => {
    const response = await page.goto(`/my-availability/${VALID_TOKEN}`);
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(page);
  });

  test("availability container is present", async ({ page }) => {
    await page.goto(`/my-availability/${VALID_TOKEN}`);
    const container = page.locator(".availability-container");
    await expect(container).toBeVisible();
  });

  test("fantasy match cards are displayed", async ({ page }) => {
    await page.goto(`/my-availability/${VALID_TOKEN}`);
    // Wait for at least one match card to appear (may load dynamically)
    const firstCard = page.locator(".fantasy-match-card").first();
    await expect(firstCard).toBeVisible();
  });

  test("match header shows match information", async ({ page }) => {
    await page.goto(`/my-availability/${VALID_TOKEN}`);
    const matchHeader = page.locator(".match-header").first();
    await expect(matchHeader).toBeVisible();
  });

  test("invalid token shows error or redirects", async ({ page }) => {
    const response = await page.goto("/my-availability/Invalid_Token_Here");
    // Should get a non-200 response or an error page
    const status = response?.status();
    // Either a 404/401/302 or an error message on the page
    if (status === 200) {
      // If it renders a page, it should show an error message
      const body = await page.textContent("body");
      expect(body).toMatch(/not found|invalid|error|unauthorized/i);
    } else {
      expect(status).toBeDefined();
    }
  });
});
