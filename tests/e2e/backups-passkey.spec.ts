import { test, expect } from "./fixtures/test-fixtures";
import { createUser, enrolPasskeyFromLink, newDevice, uniqueUsername } from "./helpers/accounts";
import { readFileSync } from "fs";

// Passkey-enabled project: see docker-compose.yml `e2e-auth`.
// An administrator who has a passkey confirms it's them with the passkey
// instead of typing a password, then downloads a backup.

test("confirm with a passkey, then download a backup", async ({ adminPage, browser }) => {
  const username = uniqueUsername("pk.backup");
  const link = await createUser(adminPage, username, "admin");
  const { context, page } = await newDevice(browser);
  try {
    await enrolPasskeyFromLink(page, link, "Backup passkey");
    await page.goto("/admin/league/backups");
    await expect(page.locator('[data-testid="backup-confirmed"]')).toHaveCount(0);

    await page.locator('[data-passkey-reauth][data-reauth-for="backup"]').click();
    // Back on the backups page, with the confirmation shown and no password asked for.
    await expect(page.locator('[data-testid="backup-confirmed"]')).toBeVisible();
    await expect(page).toHaveURL(/\/admin\/league\/backups/);
    await expect(page.locator("#current_password")).toHaveCount(0);

    const [download] = await Promise.all([
      page.waitForEvent("download"),
      page.locator('[data-testid="backup-download"]').click(),
    ]);
    expect(download.suggestedFilename()).toMatch(/^jim-dot-tennis-backup-.*\.tar\.gz$/);
    const bytes = readFileSync(await download.path());
    expect(bytes.length).toBeGreaterThan(0);
    expect(bytes[0]).toBe(0x1f);
    expect(bytes[1]).toBe(0x8b);
  } finally {
    await context.close();
  }
});
