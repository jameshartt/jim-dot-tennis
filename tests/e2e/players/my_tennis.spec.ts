import { test, expect } from "../fixtures/test-fixtures";
import { expectNoErrorBanner } from "../helpers/assertions";

const VALID_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";
const PLAYER_ID = "p-alice";

// Seeded 'My Tennis' answers for p-alice — see tests/e2e/fixtures/seed.sql.
// These strings appear in the admin summary but MUST NOT appear on
// player-facing token surfaces (see my_tennis_privacy.spec.ts).
const SEEDED_ONE_LINER = "Crafty baseliner who plays better after coffee.";
const SEEDED_HERO = "Sabalenka — power and joy";

test.describe("My Tennis — CTA on availability page", () => {
  test("availability page shows a CTA to the My Tennis form", async ({
    page,
  }) => {
    await page.goto(`/my-availability/${VALID_TOKEN}`);
    const cta = page.locator('[data-testid="my-tennis-cta"]');
    await expect(cta).toBeVisible();
    const href = await cta.getAttribute("href");
    expect(href).toBe(`/my-profile/${VALID_TOKEN}`);
    await expect(cta).toContainText(/tennis/i);
  });
});

test.describe("My Tennis — write-only GET contract (tier-aware)", () => {
  test("GET ?edit=N renders a blank tier even after a POST wrote values", async ({
    page,
  }) => {
    const unique = `ritual-${Date.now()}`;
    // POST a single tier-6 field via the token URL (pre_match_ritual lives there).
    const res = await page.request.post(`/my-profile/${VALID_TOKEN}`, {
      form: {
        tier: "6",
        intent: "finish",
        pre_match_ritual: unique,
      },
    });
    expect(res.status()).toBe(200);

    // Open tier 6 via the re-edit affordance — it must be blank, not pre-filled.
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=6`);
    const ritualInput = page.locator('input[name="pre_match_ritual"]');
    await expect(ritualInput).toBeVisible();
    await expect(ritualInput).toHaveValue("");
    const html = await page.content();
    expect(html).not.toContain(unique);
  });

  test("GET ?edit=6 does NOT echo seeded stored tier-6 preferences", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=6`);
    const oneLiner = page.locator('input[name="my_tennis_in_one_line"]');
    await expect(oneLiner).toBeVisible();
    await expect(oneLiner).toHaveValue("");
    const heroInput = page.locator('input[name="tennis_hero_or_style"]');
    await expect(heroInput).toHaveValue("");
    const html = await page.content();
    expect(html).not.toContain(SEEDED_ONE_LINER);
    expect(html).not.toContain(SEEDED_HERO);
  });
});

test.describe("My Tennis — confirmation page", () => {
  test("confirmation echoes only just-submitted fields, not stored state", async ({
    page,
  }) => {
    const unique = `signature-${Date.now()}`;
    const res = await page.request.post(`/my-profile/${VALID_TOKEN}`, {
      form: {
        tier: "3",
        intent: "finish",
        signature_shot: unique,
      },
    });
    expect(res.status()).toBe(200);
    const html = await res.text();

    // Must include the just-submitted value.
    expect(html).toContain(unique);
    // Must NOT include unrelated seeded stored answers.
    expect(html).not.toContain(SEEDED_ONE_LINER);
    expect(html).not.toContain(SEEDED_HERO);

    // data-testids on the confirmation page are present.
    expect(html).toContain('data-testid="confirmation-heading"');
    expect(html).toContain('data-testid="submitted-fields"');
    expect(html).toContain('data-testid="update-another"');
    expect(html).toContain('data-testid="back-to-availability"');
  });
});

test.describe("My Tennis — merge semantics (round-trip via admin)", () => {
  // Serial: both tests share state on p-alice.
  test.describe.configure({ mode: "serial" });

  test("a partial POST updates one field without clearing others", async ({
    adminPage,
    page,
  }) => {
    const firstValue = `round-trip-first-${Date.now()}`;
    const secondValue = `round-trip-second-${Date.now() + 1}`;

    // 1) Write field A (walkout_song), tier 6.
    let res = await page.request.post(`/my-profile/${VALID_TOKEN}`, {
      form: {
        tier: "6",
        intent: "finish",
        walkout_song: firstValue,
      },
    });
    expect(res.status()).toBe(200);

    // 2) Write field B (tennis_spirit_animal) in a separate POST, same tier.
    //    If merge semantics are correct, field A is preserved.
    res = await page.request.post(`/my-profile/${VALID_TOKEN}`, {
      form: {
        tier: "6",
        intent: "finish",
        tennis_spirit_animal: secondValue,
      },
    });
    expect(res.status()).toBe(200);

    // 3) Verify via the admin surface that BOTH values were preserved,
    //    and the seeded one-liner (untouched by our POSTs) is still there.
    await adminPage.goto(`/admin/league/players/${PLAYER_ID}/edit`);
    await expectNoErrorBanner(adminPage);
    const summary = adminPage.locator('[data-testid="my-tennis-summary"]');
    await expect(summary).toBeVisible();
    const summaryText = (await summary.textContent()) ?? "";
    expect(
      summaryText,
      "merge POST cleared an earlier field's value",
    ).toContain(firstValue);
    expect(summaryText).toContain(secondValue);
    expect(
      summaryText,
      "merge POST clobbered a field it never touched",
    ).toContain(SEEDED_ONE_LINER);
  });

  test("explicit __clear empties a multi-select without touching other fields", async ({
    adminPage,
    page,
  }) => {
    // Seeded improvement_focus = ["serve","volleys"].
    // POST the explicit clear — repo should set the column to `[]`.
    const res = await page.request.post(`/my-profile/${VALID_TOKEN}`, {
      form: {
        tier: "5",
        intent: "finish",
        __clear_improvement_focus: "1",
      },
    });
    expect(res.status()).toBe(200);

    await adminPage.goto(`/admin/league/players/${PLAYER_ID}/edit`);
    const summary = adminPage.locator('[data-testid="my-tennis-summary"]');
    await expect(summary).toBeVisible();
    const summaryText = (await summary.textContent()) ?? "";
    // Seeded one-liner (unrelated field) should still be there.
    expect(summaryText).toContain(SEEDED_ONE_LINER);
  });
});

test.describe("My Tennis — admin summary surface", () => {
  test("player edit page shows the My Tennis summary for seeded player", async ({
    adminPage,
  }) => {
    await adminPage.goto(`/admin/league/players/${PLAYER_ID}/edit`);
    await expectNoErrorBanner(adminPage);
    const summary = adminPage.locator('[data-testid="my-tennis-summary"]');
    await expect(summary).toBeVisible();
    // Seeded one-liner is the clearest canary for "preferences were loaded".
    await expect(summary).toContainText(SEEDED_ONE_LINER);
  });
});
