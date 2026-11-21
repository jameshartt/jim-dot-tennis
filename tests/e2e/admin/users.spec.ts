import { test, expect } from "../fixtures/test-fixtures";
import { createUser, uniqueUsername } from "../helpers/accounts";
import {
  expectNoErrorBanner,
  expectTitleContains,
} from "../helpers/assertions";

// Runs in the passkey-enabled "auth" project (http://localhost:8080): see
// AUTH_SPECS in playwright.config.ts for why.
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
    const usersTable = adminPage.locator(".user-table:has(tr[data-user-id])");
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

  test("creating a user gives an invite link and a pending-link row", async ({ adminPage }) => {
    const username = uniqueUsername("users.create");
    const link = await createUser(adminPage, username, "captain");
    expect(link).toContain("/account/link/");
    await expect(adminPage.locator("[data-issued-link]")).toHaveAttribute("data-link-kind", "invite");
    // The user is in the table as a captain, and the link is listed as pending.
    const row = adminPage.locator(`tr[data-username="${username}"]`);
    await expect(row).toHaveCount(1);
    await expect(row.locator("select[data-user-role-select]")).toHaveValue("captain");
    await expect(
      adminPage.locator(`[data-pending-links] tr[data-link-username="${username}"]`),
    ).toHaveCount(1);
  });

  test("changing a role works and both changes reach the audit log", async ({ adminPage }) => {
    const username = uniqueUsername("users.role");
    await createUser(adminPage, username, "captain");

    const select = () =>
      adminPage.locator(`tr[data-username="${username}"] select[data-user-role-select]`);
    // The select submits its form on change; wait for that POST and the
    // page it redirects to before doing anything else.
    const change = async (role: string) => {
      await Promise.all([
        adminPage.waitForResponse(
          (r) => /\/users\/\d+\/role$/.test(r.url()) && r.request().method() === "POST",
        ),
        select().selectOption(role),
      ]);
      await adminPage.waitForLoadState("load");
      await expect(select()).toHaveValue(role);
    };
    await change("admin");
    await change("captain");

    await adminPage.goto("/admin/league/audit-log");
    await expect(
      adminPage.locator('[data-testid="audit-entry"][data-action="user.create"]').filter({ hasText: username }),
    ).toHaveCount(1);
    await expect(
      adminPage.locator('[data-testid="audit-entry"][data-action="link.create"]').filter({ hasText: username }),
    ).toHaveCount(1);
    await expect(
      adminPage.locator('[data-testid="audit-entry"][data-action="user.role_change"]').filter({ hasText: username }),
    ).toHaveCount(2);
  });
});
