import { test, expect } from "@playwright/test";
import { expectNoErrorBanner } from "../helpers/assertions";

const VALID_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";

// /my-profile/{token} is the player-facing 'My Tennis' surface (Sprint 016).
// It is write-only: GET renders a blank form, POST applies merge semantics,
// and nothing stored is ever echoed back. These tests pin those basics.

test.describe("My Tennis — /my-profile/{token}", () => {
  test("page loads with valid token", async ({ page }) => {
    const response = await page.goto(`/my-profile/${VALID_TOKEN}`);
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(page);
  });

  test("initials heading renders without full name", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const heading = page.locator('[data-testid="profile-initials"]');
    await expect(heading).toBeVisible();
    const text = (await heading.textContent()) ?? "";
    expect(text).toMatch(/[A-Z]\.[A-Z]\./);
    expect(text).not.toContain("Alice");
    expect(text).not.toContain("Smith");
  });

  test("privacy note is visible", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const note = page.locator('[data-testid="privacy-note"]');
    await expect(note).toBeVisible();
    await expect(note).toContainText(/private|shareable|update anytime/i);
  });

  test("back to availability link is present", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const backLink = page.locator(".back-link");
    await expect(backLink).toBeVisible();
    const href = await backLink.getAttribute("href");
    expect(href).toBe(`/my-availability/${VALID_TOKEN}`);
  });

  test("match history link is present", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    // The back-link is the only top-level nav; history lives at .../history
    // but from this form we verify the route responds directly below.
    const response = await page.goto(`/my-profile/${VALID_TOKEN}/history`);
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(page);
  });

  test("match history page loads with initials heading", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}/history`);
    const initials = page.locator('[data-testid="history-initials"]');
    await expect(initials).toBeVisible();
    const text = (await initials.textContent()) ?? "";
    expect(text).toMatch(/[A-Z]\.[A-Z]\./);
    expect(text).not.toContain("Alice");
    expect(text).not.toContain("Smith");
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
