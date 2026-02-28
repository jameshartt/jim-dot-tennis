import { test, expect } from "../fixtures/test-fixtures";
import { expectNoErrorBanner } from "../helpers/assertions";

const adminSections = [
  { name: "Players", url: "/admin/league/players" },
  { name: "Fixtures", url: "/admin/league/fixtures" },
  { name: "Teams", url: "/admin/league/teams" },
  { name: "Clubs", url: "/admin/league/clubs" },
  { name: "Seasons", url: "/admin/league/seasons" },
  { name: "Users", url: "/admin/league/users" },
  { name: "Sessions", url: "/admin/league/sessions" },
  { name: "Points Table", url: "/admin/league/points-table" },
  { name: "Match Card Import", url: "/admin/league/match-card-import" },
  { name: "Selection Overview", url: "/admin/league/selection-overview" },
  { name: "Preferred Names", url: "/admin/league/preferred-names" },
  { name: "Wrapped", url: "/admin/league/wrapped" },
];

test.describe("Admin Navigation", () => {
  for (const section of adminSections) {
    test(`${section.name} page loads without errors`, async ({
      adminPage,
    }) => {
      const response = await adminPage.goto(section.url);
      expect(response?.status()).toBe(200);
      await expectNoErrorBanner(adminPage);
    });
  }

  test("away teams page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/teams/away");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });

  test("season setup page loads", async ({ adminPage }) => {
    const response = await adminPage.goto(
      "/admin/league/seasons/setup?id=1",
    );
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });

  test("fixture week overview loads", async ({ adminPage }) => {
    const response = await adminPage.goto(
      "/admin/league/fixtures/week-overview",
    );
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });
});
