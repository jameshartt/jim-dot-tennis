import { test, expect } from "../fixtures/test-fixtures";
import { expectNoErrorBanner } from "../helpers/assertions";

// A 1x1 transparent PNG.
const PNG_1PX = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
  "base64",
);

const SVG = Buffer.from(
  '<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>',
);

// These tests change the instance's display name and logo, which other specs
// read, so they run serially and put everything back afterwards.
test.describe.serial("Admin Club Settings", () => {
  test.afterAll(async ({ browser }) => {
    const context = await browser.newContext({
      storageState: "auth-state.json",
    });
    const page = await context.newPage();
    await page.goto("/admin/league/settings");
    await page.fill("#display_name", "");
    const remove = page.locator("#remove_logo");
    if ((await remove.count()) > 0) await remove.check();
    await page.click("#save-settings");
    await context.close();
  });

  test("settings page loads for the admin", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/settings");
    expect(response?.status()).toBe(200);
    await expect(adminPage.locator("h1")).toContainText("Club Settings");
    await expectNoErrorBanner(adminPage);
    await expect(adminPage.locator("#bhplta_club_id")).toBeVisible();
    await expect(adminPage.locator("#public_url")).toBeVisible();
  });

  test("dashboard links to settings", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/dashboard");
    await expect(
      adminPage.locator('a[href="/admin/league/settings"]'),
    ).toBeVisible();
  });

  test("SVG logo is rejected", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/settings");
    await adminPage.setInputFiles("#logo", {
      name: "logo.png",
      mimeType: "image/png",
      buffer: SVG,
    });
    await adminPage.click("#save-settings");
    await expect(adminPage.locator(".alert-error")).toContainText(
      "PNG, JPEG or WebP",
    );
  });

  test("name and logo show on the week overview", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/settings");
    await adminPage.fill("#display_name", "E2E Display Club");
    await adminPage.setInputFiles("#logo", {
      name: "logo.png",
      mimeType: "image/png",
      buffer: PNG_1PX,
    });
    await adminPage.click("#save-settings");
    await expect(adminPage.locator(".alert-success")).toContainText(
      "Settings saved",
    );
    const logoSrc = await adminPage
      .locator("#logo-preview")
      .getAttribute("src");
    expect(logoSrc).toMatch(/^\/branding\/logo-[0-9a-f]{16}\.png$/);

    await adminPage.goto("/admin/league/fixtures/week-overview");
    await expect(adminPage.locator(".header h1")).toContainText(
      "E2E Display Club",
    );
    await expect(adminPage.locator(`.logo img[src="${logoSrc}"]`)).toHaveCount(
      1,
    );

    const logo = await adminPage.request.get(logoSrc!);
    expect(logo.status()).toBe(200);
    expect(logo.headers()["content-type"]).toBe("image/png");
    expect(logo.headers()["x-content-type-options"]).toBe("nosniff");

    const manifest = await adminPage.request.get("/static/manifest.json");
    expect((await manifest.json()).name).toBe("E2E Display Club");
  });

  // WI-145: the e2e app has no bundled tournament stack (no
  // COURTHIVE_SERVICE_PASSWORD), so switching Tournaments on works as before
  // and says the bundled software is not switched on here. The switch is put
  // back as it was.
  test("switching Tournaments on without the bundled stack shows a note", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/settings");
    const box = adminPage.locator("#feature_tournaments");
    const wasOn = await box.isChecked();
    try {
      if (wasOn) {
        await box.uncheck();
        await adminPage.click("#save-settings");
        await expect(adminPage.locator(".alert-success")).toContainText("Settings saved");
      }
      // No tournament code is asked for: there is nothing to set up.
      await expect(adminPage.locator("#tournament_abbreviation")).toHaveCount(0);
      await adminPage.locator("#feature_tournaments").check();
      await adminPage.click("#save-settings");
      const note = adminPage.locator('[data-tournament-setup-outcome="external"]');
      await expect(note).toBeVisible();
      await expect(note.locator("[data-setup-not-configured]")).toContainText(
        "not switched on in this deployment",
      );
      await expect(note).toContainText("operations guide");
      await expect(adminPage.locator("#feature_tournaments")).toBeChecked();

      // The tournaments page shows the set-up panel in the same state, with
      // nothing to run.
      const response = await adminPage.goto("/admin/league/tournaments");
      expect(response?.status()).toBe(200);
      const panel = adminPage.locator("#tournament-setup");
      await expect(panel).toHaveAttribute("data-tournament-setup", "not-configured");
      await expect(panel).toContainText("not switched on in this deployment");
      await expect(panel.locator("[data-setup-run]")).toHaveCount(0);
    } finally {
      if (!wasOn) {
        await adminPage.goto("/admin/league/settings");
        await adminPage.locator("#feature_tournaments").uncheck();
        await adminPage.click("#save-settings");
      }
    }
  });
});
