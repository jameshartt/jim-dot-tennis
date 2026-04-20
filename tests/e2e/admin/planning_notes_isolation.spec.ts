// Sprint 017 WI-107: LEAK REGRESSION for captain_player_notes.
//
// Captain notes are an admin-only surface. They must appear on:
//   - /admin/league/planning           (dashboard popover)
//   - /admin/league/players/{id}/edit  (private section on admin player detail)
//
// They must NEVER appear on:
//   - /my-profile/{token}
//   - /my-profile/{token}/history
//   - /my-availability/{token}
//
// This spec is the CI gate on that invariant. If it fails, do NOT relax it —
// the leak is the bug.

import { test, expect } from "../fixtures/test-fixtures";

const FANTASY_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";
const CANARY = "CAPTAIN_NOTE_LEAK_CANARY_2026";

test.describe("Captain notes privacy", () => {
  test("admin planning dashboard can surface the notes popover", async ({
    adminPage,
  }) => {
    await adminPage.goto("/admin/league/planning?week=1");
    const noteIcon = adminPage.locator('[data-testid="note-icon-p-alice"]');
    await expect(noteIcon).toBeVisible();
    await noteIcon.click();
    const slot = adminPage.locator('[data-testid="captain-notes-slot"]');
    await expect(slot).toContainText(CANARY, { timeout: 5000 });
  });

  test("admin player edit page shows captain notes", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/players/p-alice/edit");
    const body = await adminPage.textContent("body");
    expect(body).toContain(CANARY);
  });

  test("/my-profile/{token} does NOT leak captain notes", async ({ page }) => {
    const resp = await page.goto(`/my-profile/${FANTASY_TOKEN}`);
    expect(resp?.status()).toBe(200);
    const body = (await page.textContent("body")) ?? "";
    expect(body).not.toContain(CANARY);
  });

  test("/my-profile/{token}/history does NOT leak captain notes", async ({
    page,
  }) => {
    const resp = await page.goto(`/my-profile/${FANTASY_TOKEN}/history`);
    // History endpoint may 200 or 404 depending on data; either way the
    // canary must never appear.
    const body = (await page.textContent("body")) ?? "";
    expect(body).not.toContain(CANARY);
    expect(resp?.status()).toBeLessThan(500);
  });

  test("/my-availability/{token} does NOT leak captain notes", async ({
    page,
  }) => {
    const resp = await page.goto(`/my-availability/${FANTASY_TOKEN}`);
    expect(resp?.status()).toBe(200);
    const body = (await page.textContent("body")) ?? "";
    expect(body).not.toContain(CANARY);
  });

  test("unauthenticated GET of /admin/league/captain-notes is blocked", async ({
    browser,
  }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    const resp = await page.goto(
      "/admin/league/captain-notes?player_id=p-alice",
    );
    // Either redirected to login OR returned a 401/403 — never a 200 body
    // containing the canary.
    const body = (await page.textContent("body")) ?? "";
    expect(body).not.toContain(CANARY);
    const status = resp?.status() ?? 0;
    expect(status === 200 ? page.url().includes("/login") : true).toBe(true);
    await context.close();
  });
});
