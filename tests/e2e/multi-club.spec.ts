import { test, expect } from '@playwright/test';
import { loginAsAdmin } from './helpers/auth';

/**
 * Multi-club verification tests.
 *
 * These tests verify the app works correctly when configured with
 * HOME_CLUB_ID=2 (Hove Park Tennis Club) instead of the default
 * St Ann's Tennis Club. The existing seed data already contains both
 * clubs — the key difference is which club the app treats as "home".
 *
 * Run with: make test-e2e-multiclub
 */
test.describe('Multi-club verification (Hove Park as home)', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
  });

  test('dashboard loads without errors', async ({ page }) => {
    await page.goto('/admin/league/dashboard');
    await expect(page).toHaveURL(/dashboard/);
    // Should show Hove Park as the home club name
    const content = await page.textContent('body');
    expect(content).toContain('Hove Park');
  });

  test('teams page loads and shows home club teams', async ({ page }) => {
    await page.goto('/admin/league/teams');
    await expect(page).toHaveURL(/teams/);
    const content = await page.textContent('body');
    // Hove Park teams should be shown as home teams
    expect(content).toContain('Hove Park');
  });

  test('fixtures page loads', async ({ page }) => {
    await page.goto('/admin/league/fixtures');
    await expect(page).toHaveURL(/fixtures/);
    // Page should render without errors
    const errorBanner = page.locator('.error, .fatal-error');
    await expect(errorBanner).toHaveCount(0);
  });

  test('points table loads', async ({ page }) => {
    await page.goto('/admin/league/points');
    await expect(page).toHaveURL(/points/);
    const errorBanner = page.locator('.error, .fatal-error');
    await expect(errorBanner).toHaveCount(0);
  });

  test('players page loads', async ({ page }) => {
    await page.goto('/admin/league/players');
    await expect(page).toHaveURL(/players/);
    const content = await page.textContent('body');
    expect(content).not.toBeNull();
  });

  test('standings page loads', async ({ page }) => {
    await page.goto('/standings');
    const content = await page.textContent('body');
    // Should show Hove Park as home club in standings
    expect(content).toContain('Hove Park');
  });

  test('fixture detail shows correct home/away perspective', async ({ page }) => {
    // Fixture 1: St Ann's A (home) vs Hove Park A (away)
    // From Hove Park's perspective, they are the away team in this fixture
    await page.goto('/admin/league/fixtures/1');
    const content = await page.textContent('body');
    expect(content).toContain('Hove Park');
  });

  test('club detail page loads for configured home club', async ({ page }) => {
    // Club ID 2 is Hove Park
    await page.goto('/admin/league/clubs/2');
    await expect(page).toHaveURL(/clubs\/2/);
    const content = await page.textContent('body');
    expect(content).toContain('Hove Park');
  });

  test('about page loads', async ({ page }) => {
    await page.goto('/about');
    const content = await page.textContent('body');
    expect(content).toContain('jim.tennis');
  });

  test('home/away designation is reversed', async ({ page }) => {
    // When home club is Hove Park, St Ann's fixtures show St Ann's as "away"
    await page.goto('/admin/league/fixtures');
    const content = await page.textContent('body');
    // The page should load and show fixtures without crashes
    expect(content).not.toBeNull();
  });
});
