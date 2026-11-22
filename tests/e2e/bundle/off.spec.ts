import { test, expect, Browser, BrowserContext, Page } from "@playwright/test";
import { firstRunSetUp } from "./helpers";

/**
 * The packaged deployment with the tournaments profile OFF (only the app and
 * Caddy run; none of the tournament variables exist). One serial flow against
 * one freshly started instance.
 */

const OFF_TEXT = "Tournaments are not switched on for this site.";

let ctx: BrowserContext;
let admin: Page;

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ browser }: { browser: Browser }) => {
  ctx = await browser.newContext();
  admin = await ctx.newPage();
});

test.afterAll(async () => {
  await ctx.close();
});

test.describe("Bundle with tournaments off", () => {
  test("first-run set-up ends with a signed-in administrator", async () => {
    await firstRunSetUp(admin);
    await admin.getByRole("link", { name: "Go to the dashboard" }).click();
    await expect(admin).toHaveURL(/\/admin\/league\/dashboard$/);
  });

  test("the tournament paths answer a plain 404 from the proxy", async () => {
    for (const path of [
      "/tournaments/",
      "/tournaments",
      "/tournaments/index.html",
      "/public/",
      "/public",
      "/api/courthive/factory/version",
      "/api/courthive/auth/me",
      "/socket.io/?EIO=4&transport=polling",
    ]) {
      const r = await admin.request.get(path, { maxRedirects: 0 });
      expect(r.status(), path).toBe(404);
      expect(await r.text(), path).toBe(OFF_TEXT);
    }
    // Also for someone who is not signed in.
    const anon = await ctx.browser()!.newContext();
    const r = await anon.request.get("/tournaments/");
    expect(r.status()).toBe(404);
    await anon.close();
  });

  test("an administrator sees no tournament links until Tournaments is switched on", async () => {
    const noLinks = async () => {
      for (const path of ["/", "/admin/league/dashboard"]) {
        await admin.goto(path);
        await expect(admin.locator('[data-testid="nav-tournaments"]')).toHaveCount(0);
        await expect(admin.locator('[data-testid="nav-tournament-home"]')).toHaveCount(0);
        await expect(admin.locator('a[href^="/tournaments"], a[href^="/tournament-home"], a[href^="/public/"]')).toHaveCount(0);
      }
    };
    await noLinks();
    // The tournament pages of the app are not found either.
    expect((await admin.request.get("/admin/league/tournaments")).status()).toBe(404);
  });

  test("switching Tournaments on says the bundled software is not part of this deployment", async () => {
    await admin.goto("/admin/league/settings");
    await admin.check("#feature_tournaments");
    await admin.click("#save-settings");
    await expect(admin).toHaveURL(/tournament_setup=external/);
    const note = admin.locator("#tournament-setup");
    await expect(note).toHaveAttribute("data-tournament-setup-outcome", "external");
    await expect(note.locator("[data-setup-not-configured]")).toContainText("not switched on in this deployment");
    // Nothing was set up.
    await expect(note.locator("[data-setup-steps]")).toHaveCount(0);

    await admin.goto("/admin/league/tournaments");
    const panel = admin.locator("#tournament-setup");
    await expect(panel).toHaveAttribute("data-tournament-setup", "not-configured");
    await expect(panel.locator("[data-setup-not-configured]")).toContainText("not switched on in this deployment");
    await expect(panel.locator("[data-setup-run]")).toHaveCount(0);

    // The feature switch is what shows the app's own tournament link (this
    // also proves the "no links" check above can see one when it is there).
    await admin.goto("/admin/league/dashboard");
    await expect(admin.locator('[data-testid="nav-tournaments"]')).toBeVisible();

    // The proxy still answers the tournament paths with 404: switching the
    // feature on in the app does not switch the software on.
    expect((await admin.request.get("/tournaments/")).status()).toBe(404);
    expect((await admin.request.get("/api/courthive/factory/version")).status()).toBe(404);
  });
});
