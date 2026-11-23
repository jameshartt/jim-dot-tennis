import { test, expect, Browser, BrowserContext, Page } from "@playwright/test";
import { FixtureServer, readSetupCode, sqlite } from "./helpers/fresh-install";

/**
 * The sprint's promise, end to end: an EMPTY database becomes a working club
 * instance through the setup pages alone. One serial flow against one fresh
 * instance (the `fresh-install` project runs a single worker and no retries).
 * The BHPLTA website is replaced by the saved pages in fixtures/bhplta, served
 * by FixtureServer; the real site is never contacted.
 */

const ADMIN_USER = "fresh.admin";
const ADMIN_PASSWORD = "a quiet morning on court seven";
const CLUB_DISPLAY_NAME = "Fresh Install Tennis";

let context: BrowserContext;
let page: Page;
let fixtures: FixtureServer;
let code: string;

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ browser }: { browser: Browser }) => {
  context = await browser.newContext();
  page = await context.newPage();
  fixtures = new FixtureServer();
  code = readSetupCode("app-fresh.log");
});

test.afterAll(async () => {
  await fixtures.stop();
  await context.close();
});

test.describe("Fresh install", () => {
  test("empty instance sends every page to setup and has no accounts", async () => {
    expect((await page.request.get("/healthz")).status()).toBe(200);
    await page.goto("/admin/league/dashboard");
    await expect(page).toHaveURL(/\/setup$/);
    await expect(page.locator('[data-setup-step="code"]')).toBeVisible();
    await page.goto("/");
    await expect(page).toHaveURL(/\/setup$/);
    expect(sqlite("SELECT COUNT(*) FROM users;")).toEqual(["0"]);
    expect(sqlite("SELECT COUNT(*) FROM clubs;")).toEqual(["0"]);
  });

  test("wrong setup code is rejected", async () => {
    await page.goto("/setup");
    await page.fill("#code", "ZZZZ-ZZZZ-ZZZZ");
    await page.click('button[type="submit"]');
    await expect(page.locator(".alert-error, .alert-danger, [role=alert]").first()).toContainText(
      "doesn't match",
    );
    await expect(page.locator('[data-setup-step="code"]')).toBeVisible();
    // Still locked out of the next step.
    await page.goto("/setup/admin");
    await expect(page).toHaveURL(/\/setup$/);
  });

  test("the code from the server log is accepted", async () => {
    await page.goto("/setup");
    await page.fill("#code", code);
    await page.click('button[type="submit"]');
    await expect(page).toHaveURL(/\/setup\/admin$/);
    await expect(page.locator('[data-setup-step="admin"]')).toBeVisible();
  });

  test("a password that breaks the policy is refused, nothing is created", async () => {
    await page.fill("#username", ADMIN_USER);
    await page.fill("#password", "tennisclub123");
    await page.fill("#password_confirm", "tennisclub123");
    await page.click('button[type="submit"]');
    await expect(page.locator('[data-setup-step="admin"]')).toBeVisible();
    await expect(page.locator("body")).toContainText("too common");
    expect(sqlite("SELECT COUNT(*) FROM users;")).toEqual(["0"]);
  });

  test("the first administrator is created and signed in", async () => {
    await page.fill("#username", ADMIN_USER);
    await page.fill("#password", ADMIN_PASSWORD);
    await page.fill("#password_confirm", ADMIN_PASSWORD);
    await page.click('button[type="submit"]');
    await expect(page).toHaveURL(/\/setup\/club$/);
    await expect(page.locator('[data-setup-step="club"]')).toBeVisible();
    expect(sqlite("SELECT username, role FROM users;")).toEqual([`${ADMIN_USER}|admin`]);
    // The code is used up: it cannot create a second administrator.
    const again = await page.request.post("/setup", { form: { code }, maxRedirects: 0 });
    expect(again.status()).toBe(404);
  });

  test("club step reports a failure when BHPLTA cannot be reached, and can be retried", async () => {
    // The fixture server is deliberately not running yet.
    await page.goto("/setup/club");
    await page.check('input[name="club"][value="st-anns"]');
    await page.click('button[type="submit"]:has-text("Use this club")');
    const retry = page.locator('button[type="submit"]:has-text("Try again")');
    await expect(retry).toBeVisible({ timeout: 90_000 });
    await expect(page.locator(".alert-error, .alert-danger, [role=alert]").first()).toContainText(
      "Nothing was changed",
    );
    // Nothing listens there, so the message says the server could not
    // connect at all (not that the website refused).
    await expect(page.locator(".alert-error, .alert-danger, [role=alert]").first()).toContainText(
      "Could not reach the BHPLTA fixtures page",
    );
    expect(sqlite("SELECT COUNT(*) FROM clubs;")).toEqual(["0"]);
    expect(sqlite("SELECT COUNT(*) FROM fixtures;")).toEqual(["0"]);
  });

  test("retry with the saved BHPLTA pages sets up the club", async () => {
    await fixtures.start();
    await page.click('button[type="submit"]:has-text("Try again")');
    await expect(page).toHaveURL(/\/setup\/done$/, { timeout: 90_000 });
    await expect(page.locator('[data-setup-step="done"]')).toBeVisible();
    await expect(page.locator("body")).toContainText("St Ann");

    // 2024 capture of the fixtures page, home club St Ann's (see fixtures/bhplta/README.md).
    const count = async (name: string) =>
      parseInt(
        (await page.locator(`[data-count="${name}"]`).innerText()).match(/\d+/)![0],
        10,
      );
    expect(await count("divisions")).toBe(4);
    expect(await count("teams")).toBe(41);
    expect(await count("weeks")).toBe(18);
    expect(await count("fixtures")).toBe(228);
    expect(await count("clubs")).toBe(12);
    await expect(page.locator('[data-count="teams"]')).toContainText("4 of them yours");

    // The database agrees with the page.
    expect(sqlite("SELECT COUNT(*) FROM divisions;")).toEqual(["4"]);
    expect(sqlite("SELECT COUNT(*) FROM teams;")).toEqual(["41"]);
    expect(sqlite("SELECT COUNT(*) FROM fixtures;")).toEqual(["228"]);
    expect(sqlite("SELECT COUNT(*) FROM clubs;")).toEqual(["12"]);
    // Only the saved pages were read.
    expect(fixtures.requests.some((r) => r.startsWith("/bhplta_tables/fixtures"))).toBe(true);
  });

  test("the dashboard is reachable from the done page", async () => {
    await page.click('a:has-text("Go to the dashboard")');
    await expect(page).toHaveURL(/\/admin\/league\/dashboard$/);
    await expect(page.locator("h1")).toContainText("Admin Dashboard");
    // The dashboard counts the home club's teams (4) and all clubs (12).
    await expect(page.getByRole("link", { name: "Teams 4", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Clubs 12", exact: true })).toBeVisible();
    expect((await page.request.get("/admin/league/clubs")).status()).toBe(200);
  });

  test("a settings change takes effect", async () => {
    await page.goto("/admin/league/settings");
    await expect(page.locator("h1")).toContainText("Club Settings");
    await page.fill("#display_name", CLUB_DISPLAY_NAME);
    await page.click("#save-settings");
    await expect(page.locator(".alert-success")).toContainText("Settings saved");
    await page.goto("/admin/league/fixtures/week-overview");
    await expect(page.locator(".header h1")).toContainText(CLUB_DISPLAY_NAME);
    const manifest = await page.request.get("/static/manifest.json");
    expect((await manifest.json()).name).toBe(CLUB_DISPLAY_NAME);
  });

  test("tennis players were seeded and a player link can be generated", async ({ browser }) => {
    const seeded = parseInt(sqlite("SELECT COUNT(*) FROM tennis_players;")[0], 10);
    expect(seeded).toBeGreaterThan(0);

    await page.goto("/admin/league/players/new");
    await page.fill("#first_name", "Fresh");
    await page.fill("#last_name", "Installer");
    await page.selectOption("#gender", "Women");
    await page.click('button:has-text("Create Player")');
    await expect(page).toHaveURL(/\/admin\/league\/players/);

    const players = await sqlite(
      "SELECT id FROM players WHERE first_name='Fresh' AND last_name='Installer';",
    );
    expect(players).toHaveLength(1);
    const playerID = players[0];

    await page.goto("/admin/league/players");
    const row = page.locator("tr", { hasText: "Fresh Installer" });
    const generate = row.locator("button.btn-generate-url");
    await expect(generate).toBeVisible();
    const [response] = await Promise.all([
      page.waitForResponse((r) => r.url().includes(`/players/${playerID}/generate-availability-url`)),
      generate.click(),
    ]);
    expect(response.status()).toBe(200);
    const { url } = await response.json();
    expect(url).toMatch(/\/my-availability\/[A-Za-z0-9_-]+$/);

    // The link opens for someone who is not signed in.
    const anon = await browser.newContext();
    const anonPage = await anon.newPage();
    const link = await anonPage.goto(new URL(url).pathname);
    expect(link?.status()).toBe(200);
    await expect(anonPage.locator("body")).toContainText("Set Your Availability (Initials: F.I.)");
    await anon.close();
  });

  test("setup is unreachable afterwards", async ({ browser }) => {
    const anon = await browser.newContext();
    for (const ctx of [context, anon]) {
      // /setup/done is by design still shown to the signed-in administrator for
      // a short while after setup (doneWindow); everyone else gets a 404.
      const gets = ["/setup", "/setup/admin", "/setup/club", "/setup/demo/clear"];
      if (ctx === anon) gets.push("/setup/done");
      for (const path of gets) {
        const res = await ctx.request.get(path, { maxRedirects: 0 });
        expect(res.status(), `GET ${path}`).toBe(404);
      }
      for (const path of ["/setup", "/setup/admin", "/setup/club", "/setup/demo"]) {
        const res = await ctx.request.post(path, { form: { code, username: "x", club: "st-anns" }, maxRedirects: 0 });
        expect(res.status(), `POST ${path}`).toBe(404);
      }
    }
    await anon.close();
  });

  test("the created administrator is the only account", async () => {
    await page.goto("/admin/league/users");
    const rows = page.locator(".user-table tbody tr");
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toContainText(ADMIN_USER);
    await expect(page.locator("body")).toContainText("All Users (1)");

    // Stronger than the page: ask the database.
    expect(sqlite("SELECT username, role, is_active FROM users;")).toEqual([`${ADMIN_USER}|admin|1`]);
    // No other kind of sign-in exists either (passkeys, other credentials).
    const tables = sqlite("SELECT name FROM sqlite_master WHERE type='table' AND (name LIKE '%passkey%' OR name LIKE '%credential%');");
    for (const t of tables) {
      expect(sqlite(`SELECT COUNT(*) FROM "${t}";`), t).toEqual(["0"]);
    }
  });
});
