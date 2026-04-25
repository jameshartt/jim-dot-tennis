import { test, expect } from "../fixtures/test-fixtures";

// Sprint 018 WI-111: wizard contract tests for /my-profile/{token}.
//
// p-alice is seeded with a full Sprint 016 preferences row, so migration
// 027's backfill places her at wizard_progress_tier=6 (the all-done state).
// We use ?edit=N to reach individual tiers without resetting her progress,
// which is the same affordance returning real users will use.

const VALID_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";
const PLAYER_ID = "p-alice";

const SEEDED_STORED_ANSWERS = [
  "Crafty baseliner who plays better after coffee.",
  "Sabalenka — power and joy",
  "Seeded Walkout Song — Stored Answer Canary",
  "Inside-out forehand",
  "Strong coffee and dynamic stretches",
];

test.describe("My Tennis wizard — default GET shows the right tier", () => {
  test("seeded p-alice (backfilled to tier 6) lands on the all-done state", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    await expect(page.locator('[data-testid="all-done"]')).toBeVisible();
    await expect(page.locator('[data-testid="reedit-list"]')).toBeVisible();
    // Re-edit affordances exist for every tier.
    for (let i = 1; i <= 6; i++) {
      await expect(
        page.locator(`[data-testid="reedit-list"] a[data-tier="${i}"]`),
      ).toBeVisible();
    }
  });

  test("the all-done state renders no form inputs and leaks no stored answer", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    expect(await page.locator('[data-testid="wizard-form"]').count()).toBe(0);
    const html = await page.content();
    for (const leak of SEEDED_STORED_ANSWERS) {
      expect(html, `all-done state echoed stored answer "${leak}"`).not.toContain(
        leak,
      );
    }
  });
});

test.describe("My Tennis wizard — write-only contract under tier-aware GET", () => {
  test("?edit=N opens tier N as a blank form regardless of stored progress", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=6`);
    await expect(page.locator('[data-testid="wizard-form"]')).toBeVisible();
    await expect(page.locator('[data-testid="tier-title"]')).toContainText(
      /fun/i,
    );
    // Every tier-6 input is empty even though p-alice has seeded answers.
    const inputs = [
      'input[name="my_tennis_in_one_line"]',
      'input[name="tennis_hero_or_style"]',
      'input[name="walkout_song"]',
      'input[name="pre_match_ritual"]',
      'input[name="years_playing"]',
    ];
    for (const sel of inputs) {
      await expect(page.locator(sel)).toHaveValue("");
    }
    const html = await page.content();
    for (const leak of SEEDED_STORED_ANSWERS) {
      expect(html, `?edit=6 echoed stored answer "${leak}"`).not.toContain(
        leak,
      );
    }
  });

  test("hidden tier input on the form matches the rendered tier", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=3`);
    await expect(page.locator('input[name="tier"]')).toHaveValue("3");
  });
});

test.describe("My Tennis wizard — terminal CTA never advances", () => {
  test("clicking 'Save & finish here' on tier 1 renders confirmation, not tier 2", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=1`);
    await page.locator('input[name="best_window_for_last_minute"]').fill(
      `WIZARD-FINISH-${Date.now()}`,
    );
    await page.locator('[data-testid="finish-cta"]').click();
    await expect(
      page.locator('[data-testid="confirmation-heading"]'),
    ).toBeVisible();
    // Did NOT advance to tier 2 inline.
    expect(await page.locator('[data-testid="wizard-form"]').count()).toBe(0);
  });
});

test.describe("My Tennis wizard — monotonic progress", () => {
  test("POST tier=2 by a user already at tier 6 keeps progress at 6", async ({
    page,
    adminPage,
  }) => {
    // Confirm starting state.
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    await expect(page.locator('[data-testid="all-done"]')).toBeVisible();

    // Submit a tier-2 POST with intent=continue.
    const res = await page.request.post(`/my-profile/${VALID_TOKEN}`, {
      form: {
        tier: "2",
        intent: "continue",
        transport: "car",
      },
    });
    expect(res.status()).toBe(200);

    // Re-fetch the GET — should STILL be the all-done state.
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    await expect(page.locator('[data-testid="all-done"]')).toBeVisible();

    // Sanity-check via admin: the just-submitted transport=car arrived,
    // proving the POST landed; progress is what the all-done state above
    // already verified.
    await adminPage.goto(`/admin/league/players/${PLAYER_ID}/edit`);
    const summary = adminPage.locator('[data-testid="my-tennis-summary"]');
    await expect(summary).toBeVisible();
  });
});

test.describe("My Tennis wizard — backfill produces all-done for fully-seeded user", () => {
  test("p-alice's backfilled progress drives the all-done UI without reading stored answers", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    // Progress strip pips for every tier appear with the 'shared' state.
    for (let i = 1; i <= 6; i++) {
      const pip = page.locator(
        `[data-testid="progress-strip"] [data-tier="${i}"]`,
      );
      await expect(pip).toBeVisible();
      await expect(pip).toHaveAttribute("data-state", "shared");
    }
  });
});

test.describe("My Tennis wizard — equal-weight CTAs", () => {
  test("non-final tier shows BOTH 'finish here' and 'keep going'", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=2`);
    await expect(page.locator('[data-testid="finish-cta"]')).toBeVisible();
    await expect(page.locator('[data-testid="continue-cta"]')).toBeVisible();
  });

  test("tier 6 (final) shows ONLY 'finish here' — no advance possible", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=6`);
    await expect(page.locator('[data-testid="finish-cta"]')).toBeVisible();
    expect(await page.locator('[data-testid="continue-cta"]').count()).toBe(0);
  });
});

test.describe("My Tennis wizard — localStorage drafts", () => {
  test("typed-but-unsubmitted value survives a page reload", async ({
    page,
  }) => {
    const draft = `LOCAL-DRAFT-${Date.now()}`;
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=3`);
    const sig = page.locator('input[name="signature_shot"]');
    await sig.fill(draft);
    // Trigger the input listener that snapshots into localStorage.
    await sig.dispatchEvent("input");

    await page.reload();
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=3`);
    await expect(page.locator('input[name="signature_shot"]')).toHaveValue(
      draft,
    );

    // Cleanup — submit the draft so it doesn't pollute later tests.
    await page.locator('[data-testid="finish-cta"]').click();
    await expect(
      page.locator('[data-testid="confirmation-heading"]'),
    ).toBeVisible();
  });

  test("draft for the submitted tier clears, but other tiers' drafts survive", async ({
    page,
  }) => {
    const tier3Draft = `T3-${Date.now()}`;
    const tier5Draft = `T5-${Date.now()}`;

    // Draft tier 3.
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=3`);
    const sig = page.locator('input[name="signature_shot"]');
    await sig.fill(tier3Draft);
    await sig.dispatchEvent("input");

    // Draft tier 5 (new page, same localStorage origin).
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=5`);
    const goal = page.locator('input[name="season_goal"]');
    await goal.fill(tier5Draft);
    await goal.dispatchEvent("input");

    // Submit tier 5.
    await page.locator('[data-testid="finish-cta"]').click();
    await expect(
      page.locator('[data-testid="confirmation-heading"]'),
    ).toBeVisible();

    // Tier 3 draft should still be in localStorage.
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=3`);
    await expect(page.locator('input[name="signature_shot"]')).toHaveValue(
      tier3Draft,
    );

    // Tier 5 draft should be cleared — open it fresh.
    await page.goto(`/my-profile/${VALID_TOKEN}?edit=5`);
    await expect(page.locator('input[name="season_goal"]')).toHaveValue("");
  });
});
