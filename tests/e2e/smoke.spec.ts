import { test, expect } from "@playwright/test";
import { loginAsAdmin, loginWith } from "./helpers/auth";

test.describe("Smoke tests", () => {
  test("homepage loads with 200 status", async ({ page }) => {
    const response = await page.goto("/");
    expect(response?.status()).toBe(200);
  });

  test("login page shows username and password fields", async ({ page }) => {
    await page.goto("/login");
    await expect(page.locator('input[name="username"]')).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
  });

  test("valid credentials redirect to admin dashboard", async ({ page }) => {
    await loginAsAdmin(page);
    await expect(page).toHaveURL(/\/admin\/league\/dashboard/);
  });

  test("invalid credentials show error message", async ({ page }) => {
    await loginWith(page, "testadmin", "wrongpassword");
    // Should stay on login page with an error
    await expect(page).toHaveURL(/\/login/);
    await expect(page.locator("body")).toContainText(/invalid|incorrect|error/i);
  });

  test("unauthenticated admin access redirects to login", async ({ page }) => {
    const response = await page.goto("/admin/league/dashboard");
    // Should redirect to /login
    expect(page.url()).toContain("/login");
  });
});
