import { Page } from "@playwright/test";

/** Navigate to the admin dashboard. */
export async function goToDashboard(page: Page): Promise<void> {
  await page.goto("/admin/league/dashboard");
}

/** Navigate to admin players list. */
export async function goToPlayers(page: Page): Promise<void> {
  await page.goto("/admin/league/players");
}

/** Navigate to admin fixtures list. */
export async function goToFixtures(page: Page): Promise<void> {
  await page.goto("/admin/league/fixtures");
}

/** Navigate to admin teams list. */
export async function goToTeams(page: Page): Promise<void> {
  await page.goto("/admin/league/teams");
}

/** Navigate to admin divisions list. */
export async function goToDivisions(page: Page): Promise<void> {
  await page.goto("/admin/league/divisions");
}

/** Navigate to admin clubs list. */
export async function goToClubs(page: Page): Promise<void> {
  await page.goto("/admin/league/clubs");
}
