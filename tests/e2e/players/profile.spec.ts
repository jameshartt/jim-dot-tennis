import { test, expect } from "@playwright/test";
import { expectNoErrorBanner } from "../helpers/assertions";

const VALID_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";

test.describe("Player Profile", () => {
  test("profile page loads with valid token", async ({ page }) => {
    const response = await page.goto(`/my-profile/${VALID_TOKEN}`);
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(page);
  });

  test("profile container is present", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const container = page.locator(".profile-container");
    await expect(container).toBeVisible();
  });

  test("player name is displayed", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const playerName = page.locator(".player-name");
    await expect(playerName).toBeVisible();
    const nameText = await playerName.textContent();
    // Should show one of the linked player's names
    expect(nameText).toBeTruthy();
  });

  test("back to availability link is present", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const backLink = page.locator(".back-link");
    await expect(backLink).toBeVisible();
    const href = await backLink.getAttribute("href");
    expect(href).toContain(`/my-availability/${VALID_TOKEN}`);
  });

  test("current season teams section is shown", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const pageContent = await page.textContent("body");
    expect(pageContent).toContain("Current Season Teams");
  });

  test("match history link is present", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const historyLink = page.locator(
      `a[href="/my-profile/${VALID_TOKEN}/history"]`,
    );
    await expect(historyLink).toBeVisible();
  });

  test("match history page loads", async ({ page }) => {
    const response = await page.goto(
      `/my-profile/${VALID_TOKEN}/history`,
    );
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(page);
  });

  test("availability statistics are displayed", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const statsGrid = page.locator(".stats-grid");
    await expect(statsGrid).toBeVisible();
  });

  test("invalid token returns error", async ({ page }) => {
    const response = await page.goto("/my-profile/Invalid_Token_Here");
    const status = response?.status();
    if (status === 200) {
      const body = await page.textContent("body");
      expect(body).toMatch(/not found|invalid|error|unauthorized/i);
    } else {
      expect(status).toBeDefined();
    }
  });
});
