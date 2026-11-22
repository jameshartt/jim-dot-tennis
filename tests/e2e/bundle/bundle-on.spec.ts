import { test, expect, Browser, BrowserContext, Page } from "@playwright/test";
import {
  ADMIN_PASSWORD,
  ADMIN_USER,
  CLUB_CODE,
  authMe,
  firstRunSetUp,
  pickDate,
  recordHosts,
  signInWithPassword,
  signOut,
  testOrigin,
  tmxTokens,
} from "./helpers";

/**
 * The packaged deployment with the tournaments profile ON, one serial flow
 * against one freshly started instance (the shipped deploy/docker-compose.yml
 * and deploy/Caddyfile; see scripts/test-e2e-bundle.sh). Steps a to k of
 * WI-147, in the order that works from an empty instance. Nothing here is
 * seeded: the administrator, the club and the tournament are all made through
 * the product's own pages.
 *
 * TMX workarounds (each one is explained where it is used, and the problem
 * behind it has its own test.fixme at the end):
 *   - TMX's "save locally" setting is switched on before the tournament is
 *     created, because after the hand-over TMX cannot open a tournament that
 *     is only on the server ("Tournament not found").
 *   - The publishing switch is clicked with a script click, because after the
 *     hand-over TMX greys the whole Publishing tab out ("Log in to manage
 *     publishing").
 */

const TOURNAMENT_NAME = "Bundle Smoke Cup";
const TOURNAMENTS_ONLY_USER = "bundle.tournaments";
const TOURNAMENTS_ONLY_PASSWORD = "an unhurried afternoon of doubles";

// The refused routes of design section 4 and what the proxy must answer.
const REFUSED = [
  ["POST", "/api/courthive/auth/login"],
  ["POST", "/api/courthive/auth/register"],
  ["POST", "/api/courthive/auth/magic/request"],
  ["POST", "/api/courthive/auth/magic/consume"],
  ["POST", "/api/courthive/auth/complete-first-login"],
  ["POST", "/api/courthive/auth/allusers"],
  ["POST", "/api/courthive/auth/invite"],
  ["GET", "/api/courthive/account/me"],
  ["POST", "/api/courthive/account/password"],
  ["GET", "/api/courthive/admin/users"],
  ["POST", "/api/courthive/admin/providers"],
  ["POST", "/api/courthive/provisioner/providers"],
  ["GET", "/api/courthive/internal/health"],
  ["GET", "/api/courthive/.well-known/jwks.json"],
  ["GET", "/api/courthive/health"],
  ["GET", "/api/courthive/health/ready"],
] as const;

let adminCtx: BrowserContext;
let admin: Page;
let tournamentId = "";
let tournamentsOnlyLink = "";

test.describe.configure({ mode: "serial" });

test.beforeAll(async ({ browser }: { browser: Browser }) => {
  adminCtx = await browser.newContext();
  admin = await adminCtx.newPage();
});

test.afterAll(async () => {
  await adminCtx.close();
});

test.describe("Bundle with tournaments on", () => {
  test("a. first-run set-up ends with a signed-in administrator", async () => {
    expect((await admin.request.get("/healthz")).status()).toBe(200);
    await admin.goto("/");
    await expect(admin).toHaveURL(/\/setup$/);
    await firstRunSetUp(admin);
    await admin.getByRole("link", { name: "Go to the dashboard" }).click();
    await expect(admin).toHaveURL(/\/admin\/league\/dashboard$/);
    await expect(admin.locator("h1")).toContainText("Admin Dashboard");
    // Tournaments start off on a new install: nothing is half switched on.
    await expect(admin.locator('[data-testid="nav-tournaments"]')).toHaveCount(0);
  });

  test("b. switching Tournaments on sets the club up; running it again changes nothing", async () => {
    await admin.goto("/admin/league/settings");
    await admin.check("#feature_tournaments");
    await admin.click("#save-settings");
    await expect(admin).toHaveURL(/tournament_setup=done/);

    const panel = admin.locator("#tournament-setup");
    await expect(panel).toHaveAttribute("data-tournament-setup-outcome", "done");
    await expect(panel.locator("[data-setup-outcome='done']")).toContainText(CLUB_CODE);
    await expect(panel.locator("[data-setup-failed]")).toHaveCount(0);
    const steps = panel.locator("[data-setup-steps] li");
    expect(await steps.count()).toBeGreaterThanOrEqual(5);
    for (const text of await steps.allInnerTexts()) {
      expect(text, "every set-up step reports success").toMatch(/: (Done|Made)/);
    }

    // The Tournaments page agrees.
    await admin.goto("/admin/league/tournaments");
    const tpanel = admin.locator("#tournament-setup");
    await expect(tpanel).toHaveAttribute("data-tournament-setup", "set-up");
    await expect(tpanel.locator("[data-setup-abbreviation]")).toHaveText(CLUB_CODE);

    // Again: everything is already there and nothing is made.
    await tpanel.locator("[data-setup-run]").click();
    const again = admin.locator("#tournament-setup");
    await expect(again).toHaveAttribute("data-tournament-setup", "set-up");
    await expect(again.locator("[data-setup-outcome='done']")).toContainText("nothing was changed");
    const againSteps = await again.locator("[data-setup-steps] li").allInnerTexts();
    expect(againSteps.length).toBeGreaterThanOrEqual(5);
    for (const text of againSteps) {
      expect(text).toMatch(/: (Done|Already there)/);
      expect(text).not.toMatch(/: Made/);
    }
  });

  test("c. 'Open tournament management' lands in TMX, signed in", async () => {
    await admin.goto("/admin/league/dashboard");
    await admin.locator('[data-testid="nav-tournament-home"]').click();
    await expect(admin).toHaveURL(/\/tournament-home$/);
    await admin.locator('[data-testid="tournament-management-link"]').click();

    await expect(admin).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    await expect.poll(async () => (await tmxTokens(admin)).access, { timeout: 30_000 }).not.toBeNull();
    const tokens = await tmxTokens(admin);
    expect(tokens.refresh).not.toBeNull();
    // The single-use token is not left in the address bar.
    expect(admin.url()).not.toContain("token=");
    // TMX shows the signed-in state.
    await expect(admin.getByText("Log in successful")).toBeVisible();

    const me = await authMe(admin);
    expect(me.status).toBe(200);
    expect(me.body.email).toBe(`${ADMIN_USER}@jim-dot-tennis.invalid`);
    expect(me.body.providerIds).toHaveLength(1);
    expect(Object.values(me.body.providerRoles)).toEqual(["PROVIDER_ADMIN"]);
  });

  test("d. a tournament is created in TMX and published", async () => {
    // Workaround 1: keep the tournament in TMX's local store too, so it can be
    // opened afterwards (see the fixme at the end). The setting is read when
    // TMX starts, hence the reload.
    await admin.evaluate(() => {
      const s = JSON.parse(localStorage.getItem("tmx_settings") || "{}");
      s.saveLocal = true;
      localStorage.setItem("tmx_settings", JSON.stringify(s));
    });
    await admin.goto("/tournaments/");

    await admin
      .getByRole("button", { name: "Create New Tournament" })
      .or(admin.getByRole("button", { name: "New Tournament" }))
      .first()
      .click();
    await admin.getByPlaceholder("Tournament name").fill(TOURNAMENT_NAME);
    // "Add" stays disabled until the name and both dates are set. The dates
    // must be picked in the calendar: two months ahead, so it is upcoming.
    const add = admin.locator("#saveTournamentEdits");
    await expect(add).toBeDisabled();
    expect(await pickDate(admin, 0, 13, 2)).toMatch(/^\d{4}-\d{2}-13$/);
    expect(await pickDate(admin, 1, 14, 0)).toMatch(/^\d{4}-\d{2}-14$/);
    await expect(add).toBeEnabled();

    const [save] = await Promise.all([
      admin.waitForRequest((r) => r.url().endsWith("/api/courthive/factory/save") && r.method() === "POST"),
      add.click(),
    ]);
    const saved = save.postDataJSON();
    expect(saved.tournamentRecord.tournamentName).toBe(TOURNAMENT_NAME);
    expect(saved.tournamentRecord.parentOrganisation.organisationAbbreviation).toBe(CLUB_CODE);
    tournamentId = saved.tournamentRecord.tournamentId;
    expect((await (await save.response())!.json()).success).toBe(true);

    // Open it and publish its information: the public calendar lists it then.
    // The list is redrawn after the save, and a click that lands in the middle
    // of that is lost (seen once in four runs), so click until it opens.
    await expect(async () => {
      await admin.getByText(TOURNAMENT_NAME).first().click();
      await expect(admin).toHaveURL(new RegExp(`#/tournament/${tournamentId}`), { timeout: 3_000 });
    }).toPass({ timeout: 30_000 });
    await expect(admin.getByText("Tournament not found")).toHaveCount(0);
    await admin.goto(`/tournaments/#/tournament/${tournamentId}/publishing`);
    const toggle = admin.locator(".pub-toggle input").first(); // "Tournament Information"
    await expect(toggle).toBeAttached();
    // Workaround 2: the grid is greyed out, so a real click does not reach it.
    await toggle.evaluate((el) => (el as HTMLInputElement).click());
    await expect(admin.locator(".pub-panel", { hasText: "Tournament Information" })).toContainText("Live");
  });

  test("e. the tournament is on the public pages, and nothing leaves the test origin", async ({ browser }) => {
    const anon = await browser.newContext();
    const page = await anon.newPage();
    const hosts = recordHosts(page);

    await page.goto(`/public/#/tournaments/${CLUB_CODE}`);
    await expect(page.getByText(TOURNAMENT_NAME)).toBeVisible({ timeout: 30_000 });
    await page.getByText(TOURNAMENT_NAME).click();
    await expect(page).toHaveURL(new RegExp(`/public/#/tournament/${tournamentId}`));
    await expect(page.getByRole("heading", { name: TOURNAMENT_NAME, level: 1 })).toBeVisible({ timeout: 30_000 });
    // A short grace period for late requests (fonts, flags, analytics). Not
    // networkidle: the live-update polling never lets TMX settle.
    await page.waitForTimeout(2000);

    expect([...hosts]).toEqual([testOrigin()]);
    await anon.close();
  });

  test("f. sync in the app finds it; once visible it is on the home page", async () => {
    await admin.goto("/admin/league/tournaments");
    await expect(admin.locator(".tournaments-table")).toHaveCount(0);
    await admin.getByRole("button", { name: "Sync from CourtHive" }).click();
    const row = admin.locator(".tournaments-table tbody tr", { hasText: TOURNAMENT_NAME });
    await expect(row).toBeVisible();
    await expect(row.locator(".tournament-id")).toHaveText(tournamentId);

    // Not shown on the home page until it is made visible.
    await admin.goto("/");
    await expect(admin.getByText(TOURNAMENT_NAME)).toHaveCount(0);

    await admin.goto("/admin/league/tournaments");
    const toggle = admin.locator(".tournaments-table tbody tr", { hasText: TOURNAMENT_NAME }).locator(".visibility-toggle");
    if ((await toggle.innerText()).trim() !== "Visible") await toggle.click();
    await expect(toggle).toHaveText("Visible");

    await admin.goto("/");
    const link = admin.locator("a.tournament-item", { hasText: TOURNAMENT_NAME });
    await expect(link).toBeVisible();
    await expect(link).toHaveAttribute("href", `/public/#/tournament/${tournamentId}`);
  });

  let own: BrowserContext;
  let ownPage: Page;

  /**
   * The administrator makes an account on the users page; the person opens the
   * invite link in their own browser and sets a password. Returns that browser
   * (signed in, on the page the redeem ended on).
   */
  async function inviteAndRedeem(
    browser: Browser,
    username: string,
    role: string,
    access: string,
    password: string,
  ): Promise<{ ctx: BrowserContext; page: Page }> {
    await admin.goto("/admin/league/users");
    await admin.fill("#username", username);
    await admin.selectOption("#role", { label: role });
    await admin.selectOption("#tournament_access", access);
    await admin.click("#create-user-submit");
    const link = await admin.locator("#issued-link-url").inputValue();
    expect(link).toContain("/account/link/");

    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await page.goto(new URL(link).pathname);
    await expect(page.locator("[data-link-username]")).toHaveText(username);
    await page.locator("details[data-passkey-fallback] summary").click();
    await page.fill("#new_password", password);
    await page.fill("#new_password_confirm", password);
    await page.click("#link-password-submit");
    await page.waitForURL((u) => !u.pathname.startsWith("/account/link/"));
    return { ctx, page };
  }

  /** From the app's tournament home, "Open tournament management" into TMX. */
  async function openManagement(page: Page): Promise<void> {
    await page.goto("/tournament-home");
    await page.locator('[data-testid="tournament-management-link"]').click();
    await expect(page).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    await expect.poll(async () => (await tmxTokens(page)).access, { timeout: 30_000 }).not.toBeNull();
  }

  test("g. a Tournaments only account: invite, sign in, TMX; the league is refused", async ({ browser }) => {
    // The administrator creates it with access to every tournament of the club.
    ({ ctx: own, page: ownPage } = await inviteAndRedeem(
      browser,
      TOURNAMENTS_ONLY_USER,
      "Tournaments only",
      "all",
      TOURNAMENTS_ONLY_PASSWORD,
    ));
    await expect(ownPage).toHaveURL(/\/tournament-home$/);

    // Then signs out and in again the ordinary way.
    await signOut(ownPage);
    await signInWithPassword(ownPage, TOURNAMENTS_ONLY_USER, TOURNAMENTS_ONLY_PASSWORD);
    await expect(ownPage).toHaveURL(/\/tournament-home$/);
    await expect(ownPage.locator('[data-testid="tournament-access"]')).toContainText("every tournament");

    // Opens tournament management and is signed in to TMX.
    await ownPage.locator('[data-testid="tournament-management-link"]').click();
    await expect(ownPage).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    await expect.poll(async () => (await tmxTokens(ownPage)).access, { timeout: 30_000 }).not.toBeNull();
    const me = await authMe(ownPage);
    expect(me.status).toBe(200);
    expect(me.body.email).toBe(`${TOURNAMENTS_ONLY_USER}@jim-dot-tennis.invalid`);
    // With access to all, the club's tournament is in the calendar.
    await expect(ownPage.getByText(TOURNAMENT_NAME).first()).toBeVisible({ timeout: 30_000 });

    // The league is not for this account.
    const league = await ownPage.request.get("/admin/league/dashboard", { maxRedirects: 0 });
    expect(league.status()).toBe(403);
  });

  /** After the reload TMX's calendar call must answer with no calendars at all. */
  async function expectNoTournamentsInOpenSession(page: Page): Promise<void> {
    const [calendar] = await Promise.all([
      page.waitForResponse((r) => r.url().endsWith("/api/courthive/provider/my-calendars")),
      page.goto("/tournaments/"),
    ]);
    expect(calendar.status()).toBe(200);
    const body = await calendar.json();
    expect(body.calendars ?? []).toHaveLength(0);
    await expect(page.getByText(TOURNAMENT_NAME)).toHaveCount(0);
  }

  test("h. taking the tournament access away ends it at once, in the open TMX session too", async ({ browser }) => {
    // A Tournaments only account cannot be set to "no tournaments": the users
    // page does not offer it (the account would be useless; it is disabled
    // instead, below). So this part uses a captain who was given access.
    await admin.goto("/admin/league/users");
    const tRow = admin.locator(`tr[data-username="${TOURNAMENTS_ONLY_USER}"]`);
    await expect(tRow.locator("[data-user-tournaments-select] option")).toHaveText(["Own tournaments", "All tournaments"]);

    const captain = await inviteAndRedeem(browser, "bundle.captain", "captain", "all", "a captain with a long memory");
    await expect(captain.page).toHaveURL(/\/admin\/league\/dashboard$/);
    await openManagement(captain.page);
    expect((await authMe(captain.page)).status).toBe(200);
    await expect(captain.page.getByText(TOURNAMENT_NAME).first()).toBeVisible({ timeout: 30_000 });

    await admin.goto("/admin/league/users");
    const cRow = admin.locator('tr[data-username="bundle.captain"]');
    await cRow.locator("[data-user-tournaments-select]").selectOption("none");
    await expect(admin.locator("body")).toContainText("Tournament access updated");

    // The still-open TMX session now sees no tournaments ...
    await expectNoTournamentsInOpenSession(captain.page);
    // ... and the app no longer hands the person over.
    await captain.page.goto("/tournament-home/open");
    await expect(captain.page).toHaveURL(/\/tournament-home$/);
    expect(captain.page.url()).not.toContain("/sso");
    await expect(captain.page.locator('[data-testid="tournament-home-refused"]')).toBeVisible();
    await captain.ctx.close();
  });

  test("h2. disabling the account does the same", async () => {
    await admin.goto("/admin/league/users");
    const row = admin.locator(`tr[data-username="${TOURNAMENTS_ONLY_USER}"]`);
    admin.once("dialog", (d) => d.accept());
    await row.getByRole("button", { name: "Deactivate" }).click();
    await expect(admin.locator(`tr[data-username="${TOURNAMENTS_ONLY_USER}"]`)).toContainText("Inactive");

    await expectNoTournamentsInOpenSession(ownPage);
    await ownPage.goto("/tournament-home/open", { waitUntil: "commit" });
    await expect(ownPage).toHaveURL(/\/login/);
    expect(ownPage.url()).not.toContain("/sso");
    await own.close();
  });

  test("i. signing out of the app lands on /login and clears TMX's tokens", async () => {
    await admin.goto("/tournaments/");
    await expect.poll(async () => (await tmxTokens(admin)).access).not.toBeNull();
    expect((await tmxTokens(admin)).refresh).not.toBeNull();

    await admin.goto("/admin/league/dashboard");
    await signOut(admin);
    await expect(admin).toHaveURL(/\/login/);
    const tokens = await tmxTokens(admin);
    expect(tokens).toEqual({ access: null, refresh: null });
  });

  test("j. TMX's 'Log in' goes through the app's sign-in and back, signed in", async ({ browser }) => {
    const anon = await browser.newContext();
    const page = await anon.newPage();
    await page.goto("/tournaments/");
    await page.locator("#login").click(); // the account menu
    await page.getByText("Log in", { exact: true }).click();

    // /tournament-home/open sends someone who is signed out to the sign-in page.
    await expect(page).toHaveURL(/\/login/, { timeout: 30_000 });
    await expect(page.locator("body")).toContainText(/Sign in|Login/i);
    expect(await tmxTokens(page)).toEqual({ access: null, refresh: null });

    await page.locator("details[data-passkey-fallback] summary").click();
    await page.fill('input[name="username"]', ADMIN_USER);
    await page.fill('input[name="password"]', ADMIN_PASSWORD);
    await page.click('form[action="/login"] button[type="submit"]');
    // The offer to add a passkey comes first, then the hand-over carries on.
    const next = page.locator("[data-signin-continue]");
    await page.waitForURL((u) => u.pathname !== "/login");
    if (new URL(page.url()).pathname === "/account/signin-methods") await next.click();

    await expect(page).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    await expect.poll(async () => (await tmxTokens(page)).access, { timeout: 30_000 }).not.toBeNull();
    const me = await authMe(page);
    expect(me.status).toBe(200);
    expect(me.body.email).toBe(`${ADMIN_USER}@jim-dot-tennis.invalid`);
    await anon.close();
  });

  test("k. CourtHive's own sign-in and admin routes are refused (404) through the proxy", async ({ playwright }) => {
    const api = await playwright.request.newContext({
      baseURL: process.env.BUNDLE_URL || "https://localhost:18743",
      ignoreHTTPSErrors: true,
    });
    for (const [method, path] of REFUSED) {
      const r = await api.fetch(path, { method, data: method === "POST" ? {} : undefined, maxRedirects: 0 });
      expect(r.status(), `${method} ${path}`).toBe(404);
    }
    // Not shipped in the bundle: 404 whether tournaments are on or off.
    for (const path of ["/api/rankings/anything", "/epixodic/"]) {
      expect((await api.get(path)).status(), path).toBe(404);
    }
    // What is allowed still answers: the version, and a token-less /auth/me is a 401.
    expect((await api.get("/api/courthive/factory/version")).status()).toBe(200);
    expect((await api.get("/api/courthive/auth/me")).status()).toBe(401);
    // The app's own session cookie never reaches CourtHive: a request that
    // carries only a cookie is still unauthenticated.
    const cookieOnly = await api.get("/api/courthive/auth/me", { headers: { Cookie: "session=anything" } });
    expect(cookieOnly.status()).toBe(401);
    await api.dispose();
  });

  // --- Problems found in the stack, kept here so they are not forgotten. ----

  // After the hand-over TMX has the person's tournaments (the calendar call
  // succeeds) but not a selected provider: the sign-in response carries
  // `providerIds` and `providerRoles` and no `provider`. In a browser that
  // does not hold the tournament locally, clicking a tournament in TMX's list
  // shows "Tournament not found" and TMX never asks the server for it (no
  // POST /factory/fetch). A person using a second device, or after clearing
  // browser data, cannot open their tournaments.
  test.fixme("TMX opens a tournament that is only on the server, after the hand-over", async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: await adminCtx.storageState() });
    const page = await ctx.newPage();
    await page.goto("/tournament-home/open");
    await page.getByText(TOURNAMENT_NAME).first().click();
    await expect(page.getByText("Tournament not found")).toHaveCount(0);
    await ctx.close();
  });

  // The same cause: the Publishing tab shows "Log in to manage publishing"
  // and its controls are greyed out although the person is signed in.
  test.fixme("TMX's Publishing tab is usable right after the hand-over", async () => {
    await admin.goto(`/tournaments/#/tournament/${tournamentId}/publishing`);
    await expect(admin.getByText("Log in to manage publishing")).toHaveCount(0);
  });
});
