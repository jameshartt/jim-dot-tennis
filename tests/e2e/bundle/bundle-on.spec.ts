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
 * TMX renews its session once right after the hand-over (a change in the
 * bundled TMX build), which is what gives it the club's record: the last test
 * proves a browser that has never seen the tournament can open and publish it.
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
    await expect(admin.getByText("Log in to manage publishing")).toHaveCount(0);
    await admin.locator(".pub-toggle").first().click();
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
    sets?: string[],
  ): Promise<{ ctx: BrowserContext; page: Page }> {
    await admin.goto("/admin/league/users");
    await admin.fill("#username", username);
    await admin.selectOption("#role", { label: role });
    await admin.selectOption("#tournament_access", access);
    if (sets) {
      // Only offered once the club has more than one set of tournaments.
      for (const box of await admin.locator("[data-user-create-tournament-sets] input").all()) {
        await box.setChecked(sets.includes((await box.getAttribute("data-set-code")) ?? ""));
      }
    }
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

  /**
   * After the reload TMX's calendar call must list no tournament. TMX names the
   * club it last acted for, so the answer may still carry that club's entry;
   * what matters is that the entry is empty.
   */
  async function expectNoTournamentsInOpenSession(page: Page): Promise<void> {
    const [calendar] = await Promise.all([
      page.waitForResponse((r) => r.url().endsWith("/api/courthive/provider/my-calendars")),
      page.goto("/tournaments/"),
    ]);
    expect(calendar.status()).toBe(200);
    const body = await calendar.json();
    const listed = (body.calendars ?? []).flatMap((c: { tournaments?: unknown[] }) => c.tournaments ?? []);
    expect(listed).toHaveLength(0);
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

  // A browser that has never held the tournament (a second device, or after
  // clearing browser data): TMX must fetch it from the server and let the
  // person manage its publishing. Both depend on the session carrying the
  // club's record, which the hand-over alone does not give TMX.
  test("l. a fresh browser opens a tournament from the server and can manage publishing", async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await signInWithPassword(page, ADMIN_USER, ADMIN_PASSWORD);
    await page.goto("/tournament-home/open");
    await expect(page).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    const [fetched] = await Promise.all([
      page.waitForResponse((r) => r.url().endsWith("/api/courthive/factory/fetch"), { timeout: 30_000 }),
      page.getByText(TOURNAMENT_NAME).first().click(),
    ]);
    expect(fetched.status()).toBeLessThan(300);
    await expect(page).toHaveURL(new RegExp(`#/tournament/${tournamentId}`));
    await expect(page.getByText("Tournament not found")).toHaveCount(0);

    await page.goto(`/tournaments/#/tournament/${tournamentId}/publishing`);
    await expect(page.locator(".pub-panel", { hasText: "Tournament Information" })).toContainText("Live");
    await expect(page.getByText("Log in to manage publishing")).toHaveCount(0);
    await ctx.close();
  });
  // --- Several sets of tournaments in one installation (WI-154). -----------
  // These come last: from here on the administrator has two sets, so the
  // hand-over asks which one.

  /** The provider a TMX session acts for, read from its access token. */
  async function actingProvider(page: Page): Promise<string | undefined> {
    return page.evaluate(() => {
      const token = localStorage.getItem("tmxToken");
      if (!token) return undefined;
      const payload = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
      return JSON.parse(atob(payload)).providerId as string | undefined;
    });
  }

  /**
   * The names of the tournaments the tournament server lists for the page's
   * TMX session (in one set of tournaments when providerId is given), asked
   * with TMX's own token. (Not read off TMX's own request:
   * after a switch TMX reloads its list by itself, and that races a reload
   * made here.)
   */
  async function calendarNames(page: Page, providerId?: string): Promise<string[]> {
    const answer = await page.evaluate(async () => {
      const r = await fetch("/api/courthive/provider/my-calendars", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${localStorage.getItem("tmxToken")}` },
        body: "{}",
      });
      return { status: r.status, body: await r.json() };
    });
    expect(answer.status).toBe(200);
    // Asked without naming one, the tournament server lists every set the
    // person belongs to, grouped; providerId picks one group.
    return (answer.body.calendars ?? [])
      .filter((c: { provider?: { organisationId?: string } }) => !providerId || c.provider?.organisationId === providerId)
      .flatMap((c: { tournaments?: { tournament?: { tournamentName?: string } }[] }) => c.tournaments ?? [])
      .map((t: { tournament?: { tournamentName?: string } }) => t.tournament?.tournamentName ?? "");
  }

  let clubProviderId: string | undefined;
  let cupProviderId: string | undefined;

  test("m. the administrator takes on a second set of tournaments that was made outside the app", async () => {
    await signInWithPassword(admin, ADMIN_USER, ADMIN_PASSWORD);
    await admin.goto("/admin/league/tournaments");
    const sets = admin.locator("[data-setup-sets]");
    await expect(sets.locator(`[data-setup-set="${CLUB_CODE}"]`)).toBeVisible();
    // The script made OLDCUP on the tournament server by hand: found, not managed.
    await expect(sets.locator('[data-found-code="OLDCUP"]')).toBeVisible();

    // The name people will see for the club's own set.
    await sets.locator(`[data-setup-set="${CLUB_CODE}"] input[name="name"]`).fill("St Ann's club tournaments");
    await sets.locator(`[data-setup-set="${CLUB_CODE}"] button`).click();
    await expect(admin.locator("body")).toContainText("Name changed");

    await admin.fill("#add-set-name", "Parks Cup");
    await admin.fill("#add-set-code", "oldcup");
    await admin.locator("[data-setup-add-set] button").click();
    const panel = admin.locator("#tournament-setup");
    await expect(panel.locator("[data-setup-outcome='done']")).toContainText("Parks Cup is ready");
    await expect(panel.locator("[data-setup-failed]")).toHaveCount(0);
    await expect(panel.locator('[data-setup-set="OLDCUP"] input[name="name"]')).toHaveValue("Parks Cup");
    await expect(panel.locator('[data-found-code="OLDCUP"]')).toHaveCount(0);
    for (const text of await panel.locator("[data-setup-steps] li").allInnerTexts()) {
      expect(text, "every step of adding the set reports success").toMatch(/: (Done|Made|Already there)/);
    }
  });

  test("n. someone with two sets chooses which tournaments to manage, by name", async () => {
    await admin.goto("/tournament-home");
    await expect(admin.getByRole("heading", { name: "Which tournaments do you want to manage?" })).toBeVisible();
    const choices = admin.locator('[data-testid="tournament-set"]');
    await expect(choices).toHaveCount(2);
    await expect(choices.nth(0)).toContainText("St Ann's club tournaments");
    await expect(choices.nth(1)).toContainText("Parks Cup");
    await expect(admin.locator('[data-testid="tournament-management-link"]')).toHaveCount(0);
    await expect(admin.locator('[data-testid="tournament-sets-note"]')).toBeVisible();
    // People are never shown the word "provider".
    expect((await admin.locator("main, body").first().innerText()).toLowerCase()).not.toContain("provider");

    // Accessible, at desktop and phone width; a picture of each is kept.
    const AxeBuilder = (await import("@axe-core/playwright")).default;
    for (const [name, size] of [
      ["desktop", { width: 1280, height: 800 }],
      ["mobile", { width: 375, height: 812 }],
    ] as const) {
      await admin.setViewportSize(size);
      // The whole page as the main suite checks pages (it leaves colour
      // contrast out: the site-wide link colour is a known matter there),
      // and the choice itself with every rule, contrast included.
      const pageResults = await new AxeBuilder({ page: admin })
        .withTags(["wcag2a", "wcag2aa"])
        .disableRules(["color-contrast"])
        .analyze();
      expect(pageResults.violations.map((v) => `${v.id}: ${v.help}`), `axe, page at ${name} width`).toEqual([]);
      const choiceResults = await new AxeBuilder({ page: admin })
        .withTags(["wcag2a", "wcag2aa"])
        .include('[data-testid="tournament-sets"]')
        .include("#choose")
        .include('[data-testid="tournament-sets-note"]')
        .analyze();
      expect(choiceResults.violations.map((v) => `${v.id}: ${v.help}`), `axe, the choice at ${name} width`).toEqual([]);
      await admin.screenshot({ path: `${test.info().config.rootDir}/artifacts/choice-${name}.png`, fullPage: true });
    }
    await admin.setViewportSize({ width: 1280, height: 800 });
    // The choices are real links, reachable and operable from the keyboard.
    await choices.nth(1).focus();
    await expect(choices.nth(1)).toBeFocused();

    // A link that names no set (TMX's own "Log in", a bookmark) comes here
    // instead of picking one.
    await admin.goto("/tournament-home/open");
    await expect(admin).toHaveURL(/\/tournament-home(#choose)?$/);
    expect(admin.url()).not.toContain("/sso");

    // Choosing one opens it: the session acts for that set, which has no
    // tournaments yet.
    await admin.locator('[data-testid="tournament-set"]', { hasText: "Parks Cup" }).press("Enter");
    await expect(admin).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    await expect.poll(async () => (await tmxTokens(admin)).access, { timeout: 30_000 }).not.toBeNull();
    const me = await authMe(admin);
    expect(me.status).toBe(200);
    expect(me.body.providerIds).toHaveLength(2);
    cupProviderId = await actingProvider(admin);
    expect(cupProviderId).toBeTruthy();
    expect(await calendarNames(admin, cupProviderId)).not.toContain(TOURNAMENT_NAME);
    // TMX lists the set it is acting for: the club's tournament is not there.
    // (TMX's badge shows the set's code; the home page shows that code under each name.)
    await expect(admin.locator("#provider")).toHaveText("OLDCUP", { timeout: 30_000 });
    await expect(admin.getByText(TOURNAMENT_NAME)).toHaveCount(0);

    // And the other one.
    await admin.goto("/tournament-home");
    await admin.locator('[data-testid="tournament-set"]', { hasText: "St Ann's club tournaments" }).click();
    await expect(admin).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    await expect.poll(async () => actingProvider(admin), { timeout: 30_000 }).not.toBe(cupProviderId);
    clubProviderId = await actingProvider(admin);
    expect(clubProviderId).toBeTruthy();
    expect(await calendarNames(admin, clubProviderId)).toContain(TOURNAMENT_NAME);
    await expect(admin.getByText(TOURNAMENT_NAME).first()).toBeVisible({ timeout: 30_000 });
  });

  test("o. TMX's own switcher moves between the two sets", async () => {
    // The session is acting for the club's own set (end of the last test).
    await admin.goto("/tournaments/#/tournaments");
    await expect(admin.getByText(TOURNAMENT_NAME).first()).toBeVisible({ timeout: 30_000 });
    const [selected] = await Promise.all([
      admin.waitForResponse((r) => r.url().includes("/api/courthive/auth/select-provider"), { timeout: 30_000 }),
      (async () => {
        await admin.locator("#provider").click();
        await admin.getByText("Made by hand cup", { exact: true }).click();
      })(),
    ]);
    expect(selected.status(), "the proxy lets the switch through to the identity service").toBeLessThan(300);
    await expect.poll(async () => actingProvider(admin), { timeout: 30_000 }).toBe(cupProviderId);
    expect(await calendarNames(admin, cupProviderId)).not.toContain(TOURNAMENT_NAME);
    // TMX's own list follows: the club's tournament is gone from it.
    await expect(admin.getByText(TOURNAMENT_NAME)).toHaveCount(0);
  });

  test("p. someone picked for the second set only is handed over into it and cannot reach the first", async ({ browser }) => {
    const cup = await inviteAndRedeem(browser, "bundle.cup", "Tournaments only", "all", "a long rally on the far court", ["OLDCUP"]);
    await expect(cup.page).toHaveURL(/\/tournament-home$/);
    // One set: the single button, no choice to make.
    await expect(cup.page.locator('[data-testid="tournament-set"]')).toHaveCount(0);
    await cup.page.locator('[data-testid="tournament-management-link"]').click();
    await expect(cup.page).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    await expect.poll(async () => (await tmxTokens(cup.page)).access, { timeout: 30_000 }).not.toBeNull();

    const me = await authMe(cup.page);
    expect(me.status).toBe(200);
    expect(me.body.providerIds).toEqual([cupProviderId]);
    expect(await actingProvider(cup.page)).toBe(cupProviderId);
    // Provider administrator of the cup, and still the club's tournament is not theirs to see.
    expect(await calendarNames(cup.page)).not.toContain(TOURNAMENT_NAME);
    const fetched = await cup.page.evaluate(async (id) => {
      const r = await fetch("/api/courthive/factory/fetch", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${localStorage.getItem("tmxToken")}` },
        body: JSON.stringify({ tournamentId: id }),
      });
      const text = await r.text();
      return { status: r.status, hasRecord: text.includes("Bundle Smoke Cup") };
    }, tournamentId);
    expect(fetched.hasRecord, `fetching the club's tournament answered ${fetched.status}`).toBe(false);

    // Asking the app for the club's own set is refused: no hand-over.
    await cup.page.goto(`/tournament-home/open?provider=${CLUB_CODE}`);
    await expect(cup.page).toHaveURL(/\/tournament-home$/);
    expect(cup.page.url()).not.toContain("/sso");

    // The users page shows the choice, by name.
    await admin.goto("/admin/league/users");
    const row = admin.locator('tr[data-username="bundle.cup"]');
    await expect(row).toContainText("Can manage tournaments for");
    await expect(row.locator('input[data-set-code="OLDCUP"]')).toBeChecked();
    await expect(row.locator(`input[data-set-code="${CLUB_CODE}"]`)).not.toBeChecked();

    // Given the club's own as well: the choice appears for them, and it opens.
    await row.locator(`input[data-set-code="${CLUB_CODE}"]`).check();
    await row.locator("[data-user-tournament-sets-form] button").click();
    await expect(admin.locator("body")).toContainText("Updated which tournaments they can manage");
    await cup.page.goto("/tournament-home");
    await expect(cup.page.locator('[data-testid="tournament-set"]')).toHaveCount(2);
    await cup.page.locator('[data-testid="tournament-set"]', { hasText: "St Ann's club tournaments" }).click();
    await expect(cup.page).toHaveURL(/\/tournaments\/#\//, { timeout: 30_000 });
    await expect.poll(async () => actingProvider(cup.page), { timeout: 30_000 }).toBe(clubProviderId);
    expect(await calendarNames(cup.page, clubProviderId)).toContain(TOURNAMENT_NAME);
    await cup.ctx.close();
  });
});
