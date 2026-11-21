import { test, expect } from "../fixtures/test-fixtures";
import { loginAsCaptain } from "../helpers/auth";
import { readFileSync } from "fs";

// The backups page (WI-138): administrators only; the download is
// POST /account/backup and needs the current password (or a passkey
// confirmation, see ../backups-passkey.spec.ts).

const ADMIN_PASSWORD = "testpassword123";
const BACKUP_NAME = /^jim-dot-tennis-backup-.*\.tar\.gz$/;

test.describe("Backups page", () => {
  test("an administrator sees the page, the last-backup information and the dashboard link", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/dashboard");
    const link = adminPage.locator('[data-testid="nav-backups"]');
    await expect(link).toBeVisible();
    await link.click();
    await expect(adminPage).toHaveURL(/\/admin\/league\/backups$/);
    await expect(adminPage.locator('[data-testid="backups-title"]')).toHaveText("Backups");
    await expect(adminPage.locator('[data-testid="backup-last"]')).toBeVisible();
    await expect(adminPage.locator('[data-testid="backups-table"]')).toBeVisible();
    await expect(adminPage.locator('[data-testid="backup-download-form"]')).toBeVisible();
  });

  test("a captain sees no link and gets the friendly 403 on the page", async ({ page }) => {
    await loginAsCaptain(page);
    await expect(page.locator('[data-testid="nav-backups"]')).toHaveCount(0);
    const response = await page.goto("/admin/league/backups");
    expect(response?.status()).toBe(403);
    await expect(page.locator('[data-testid="forbidden-page"]')).toBeVisible();
  });

  test("downloading with the correct password gives a gzip archive", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/backups");
    await adminPage.locator("#current_password").fill(ADMIN_PASSWORD);
    const [download] = await Promise.all([
      adminPage.waitForEvent("download"),
      adminPage.locator('[data-testid="backup-download"]').click(),
    ]);
    expect(download.suggestedFilename()).toMatch(BACKUP_NAME);
    const bytes = readFileSync(await download.path());
    expect(bytes.length).toBeGreaterThan(0);
    // gzip magic bytes
    expect(bytes[0]).toBe(0x1f);
    expect(bytes[1]).toBe(0x8b);
  });

  test("a wrong password shows an error and downloads nothing", async ({ adminPage }) => {
    // One wrong attempt only: wrong passwords count against a limit shared
    // by all forms for this address (5 per 15 minutes).
    await adminPage.goto("/admin/league/backups");
    await adminPage.locator("#current_password").fill("definitely-not-the-password");
    let downloaded = false;
    adminPage.on("download", () => {
      downloaded = true;
    });
    await adminPage.locator('[data-testid="backup-download"]').click();
    await expect(adminPage).toHaveURL(/\/admin\/league\/backups\?error=password/);
    await expect(adminPage.locator('[data-testid="backup-download-error"]')).toBeVisible();
    expect(downloaded).toBe(false);
  });
});
