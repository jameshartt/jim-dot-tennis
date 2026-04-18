import { test, expect } from "@playwright/test";

const VALID_TOKEN = "Sabalenka_Djokovic_Gauff_Sinner";

// PRIVACY SWEEP (Sprint 016, WI-097 + WI-100):
//
// Token-authenticated URLs are shareable (printed on notice boards, forwarded
// in group chats). They must not expose:
//   1) The player's legal name — only initials.
//   2) Any stored 'My Tennis' answer — GET is write-only.
//   3) Any captain-authored note about the player or their partners.
//
// The seed fixture loads a canary captain note body
// `CAPTAIN_NOTE_LEAK_CANARY_2026` that must NEVER appear on player surfaces.
// Seeded preferences include literal strings like
// `Crafty baseliner who plays better after coffee.` that the admin can see
// but the player-facing surface must not.

// The seeded subject player (p-alice = Alice Smith). Her legal name must
// never appear on any token-authenticated surface. Roster-mate names ARE
// shown in the partner picker by design (players need to recognise their
// club-mates to tick them), so we do NOT assert on those.
const SUBJECT_FULL_NAMES = ["Alice", "Smith"];
const SEEDED_STORED_ANSWERS = [
  "Crafty baseliner who plays better after coffee.",
  "Sabalenka — power and joy",
  "Seeded Walkout Song — Stored Answer Canary",
  "Happy to share a ride from central Brighton.",
  "Chronic tennis elbow flares up after three sets",
];
const CAPTAIN_NOTE_CANARY = "CAPTAIN_NOTE_LEAK_CANARY_2026";

test.describe("My Tennis — privacy sweep of /my-profile/{token}", () => {
  test("form page body contains initials but no legal name", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const body = (await page.textContent("body")) ?? "";

    // Must contain initials like "A.S."
    expect(body).toMatch(/[A-Z]\.[A-Z]\./);
    for (const leak of SUBJECT_FULL_NAMES) {
      expect(body, `player-facing form leaked "${leak}"`).not.toContain(leak);
    }
  });

  test("form page <title> does not contain the player's name", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const title = await page.title();
    for (const leak of SUBJECT_FULL_NAMES) {
      expect(title, `title leaked "${leak}"`).not.toContain(leak);
    }
  });

  test("form page never echoes any seeded stored answer", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const html = await page.content();
    for (const leak of SEEDED_STORED_ANSWERS) {
      expect(
        html,
        `GET form echoed stored answer "${leak}" — privacy contract broken`,
      ).not.toContain(leak);
    }
  });

  test("form page does not leak any captain-authored note", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}`);
    const html = await page.content();
    expect(html).not.toContain(CAPTAIN_NOTE_CANARY);
  });

  test("match history page shows initials only, no partner / opponent names", async ({
    page,
  }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}/history`);
    const body = (await page.textContent("body")) ?? "";
    for (const leak of SUBJECT_FULL_NAMES) {
      expect(body, `history page leaked "${leak}"`).not.toContain(leak);
    }
  });

  test("match history page does not leak captain notes", async ({ page }) => {
    await page.goto(`/my-profile/${VALID_TOKEN}/history`);
    const html = await page.content();
    expect(html).not.toContain(CAPTAIN_NOTE_CANARY);
  });
});

test.describe("My Tennis — privacy sweep of /my-availability/{token}", () => {
  test("availability page does not leak captain notes or stored answers", async ({
    page,
  }) => {
    await page.goto(`/my-availability/${VALID_TOKEN}`);
    const html = await page.content();
    expect(html).not.toContain(CAPTAIN_NOTE_CANARY);
    for (const leak of SEEDED_STORED_ANSWERS) {
      expect(
        html,
        `availability page leaked stored answer "${leak}"`,
      ).not.toContain(leak);
    }
  });
});
