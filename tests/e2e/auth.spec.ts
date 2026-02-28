import { test, expect } from "@playwright/test";
import { loginAsAdmin, loginWith } from "./helpers/auth";

test.describe("Authentication flows", () => {
  test("valid login redirects to admin dashboard", async ({ page }) => {
    await loginAsAdmin(page);
    await expect(page).toHaveURL(/\/admin\/league\/dashboard/);
    await expect(page.locator("h1")).toContainText("Admin Dashboard");
  });

  test("wrong password shows error and preserves username", async ({
    page,
  }) => {
    await loginWith(page, "testadmin", "wrongpassword");
    await expect(page).toHaveURL(/\/login/);
    await expect(page.locator(".alert.alert-danger")).toBeVisible();
    // Username should be preserved in the input
    await expect(page.locator("#username")).toHaveValue("testadmin");
  });

  test("empty form submission is prevented by HTML5 validation", async ({
    page,
  }) => {
    await page.goto("/login");
    const username = page.locator("#username");
    const password = page.locator("#password");
    await expect(username).toHaveAttribute("required", "");
    await expect(password).toHaveAttribute("required", "");
  });

  test("logout redirects to login and clears session", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/logout");
    await expect(page).toHaveURL(/\/login/);
    // Trying to access admin should redirect to login
    await page.goto("/admin/league/dashboard");
    await expect(page).toHaveURL(/\/login/);
  });

  test("session persists across navigation", async ({ page }) => {
    await loginAsAdmin(page);
    await page.goto("/");
    await page.goto("/admin/league/dashboard");
    await expect(page).toHaveURL(/\/admin\/league\/dashboard/);
    await expect(page.locator("h1")).toContainText("Admin Dashboard");
  });

  test("protected route redirects unauthenticated users to login", async ({
    page,
  }) => {
    await page.goto("/admin/league/dashboard");
    await expect(page).toHaveURL(/\/login/);
  });

  test("protected admin pages all redirect when unauthenticated", async ({
    page,
  }) => {
    const protectedRoutes = [
      "/admin/league/players",
      "/admin/league/fixtures",
      "/admin/league/teams",
      "/admin/league/clubs",
      "/admin/league/seasons",
      "/admin/league/users",
      "/admin/league/points-table",
    ];
    for (const route of protectedRoutes) {
      await page.goto(route);
      expect(page.url()).toContain("/login");
    }
  });
});

test.describe("Rate limiting", () => {
  test("blocks login after too many failed attempts", async ({ page }) => {
    // Use a unique username to avoid affecting other tests' login ability
    const rateUser = `ratelimit_${Date.now()}`;
    for (let i = 0; i < 6; i++) {
      await loginWith(page, rateUser, `wrongpassword${i}`);
    }
    // The last response should indicate rate limiting
    await expect(page.locator("body")).toContainText(/too many/i);
  });
});
