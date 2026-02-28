import { test, expect } from "../fixtures/test-fixtures";
import { expectNoErrorBanner } from "../helpers/assertions";

test.describe("Admin Divisions", () => {
  test("division edit page loads for seeded division", async ({
    adminPage,
  }) => {
    const response = await adminPage.goto("/admin/league/divisions/1");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });

  test("division page shows division details", async ({ adminPage }) => {
    await adminPage.goto("/admin/league/divisions/1");
    const pageContent = await adminPage.textContent("body");
    expect(pageContent).toContain("Division 1");
  });

  test("division review page loads", async ({ adminPage }) => {
    const response = await adminPage.goto("/admin/league/divisions/review");
    expect(response?.status()).toBe(200);
    await expectNoErrorBanner(adminPage);
  });
});
