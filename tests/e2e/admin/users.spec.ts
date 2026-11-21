import { test, expect } from "../fixtures/test-fixtures";
import {
  expectNoErrorBanner,
  expectTitleContains,
} from "../helpers/assertions";

test.describe("Admin Users", () => {
  test("users page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/users");
    expect(response?.status()).toBe(200);
    await expectTitleContains(adminPage, "Jim.Tennis");
    await expectNoErrorBanner(adminPage);
  });

  test("users heading is visible", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/users");
    await expect(adminPage.locator("h1")).toContainText("User Management");
  });

  test("seeded admin user is displayed", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/users");
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("testadmin");
  });

  test("users table is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/users");
    const usersTable = adminPage.locator(".user-table");
    await expect(usersTable).toBeVisible();
    const rows = usersTable.locator("tbody tr");
    const count = await rows.count();
    expect(count).toBeGreaterThanOrEqual(1); // At least testadmin
  });

  test("create user form is present", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/users");
    const usernameInput = adminPage.locator("#username");
    const roleSelect = adminPage.locator("#role");
    await expect(usernameInput).toBeVisible();
    await expect(roleSelect).toBeVisible();
    // Administrators never choose another user's password: the form makes an
    // invite link instead.
    await expect(adminPage.locator("form[data-user-create] input[type=password]")).toHaveCount(0);
  });

  test("sessions page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/sessions");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });

  test("audit log lists the administrator's sign-in, read-only", async ({
    adminPage,
  }) => {
    const response = await adminPage.goto("/admin/league/audit-log");
    expect(response?.status()).toBe(200);
    await expect(adminPage.locator('[data-testid="audit-log-title"]')).toBeVisible();
    // The shared admin session was created by a password sign-in.
    await expect(
      adminPage.locator('[data-testid="audit-entry"][data-action="user.sign_in"]').first(),
    ).toBeVisible();
    // Nothing on the page can change an entry.
    await expect(adminPage.locator('[data-testid="audit-log-table"] form')).toHaveCount(0);
  });
});
