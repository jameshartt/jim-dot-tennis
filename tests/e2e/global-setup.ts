import { chromium, FullConfig } from "@playwright/test";
import { continueAfterPasswordSignIn, openPasswordForm } from "./helpers/auth";

/**
 * Global setup: log in once as admin and save the browser storage state
 * so all test workers can reuse the session without hitting rate limits.
 */
async function globalSetup(config: FullConfig) {
  // The fresh-install run has no seeded admin: it creates its own.
  if (process.env.FRESH_INSTALL) return;

  const baseURL =
    config.projects[0].use.baseURL || "http://app-e2e:8080";

  const browser = await chromium.launch();
  const context = await browser.newContext({ baseURL });
  const page = await context.newPage();

  await page.goto("/login");
  await openPasswordForm(page);
  await page.fill('input[name="username"]', "testadmin");
  await page.fill('input[name="password"]', "testpassword123");
  await page.click('button[type="submit"]');
  await continueAfterPasswordSignIn(page);
  await page.waitForURL("**/admin/league/dashboard");

  // Save the authenticated state
  await context.storageState({ path: "auth-state.json" });
  await browser.close();
}

export default globalSetup;
