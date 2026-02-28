import { chromium, FullConfig } from "@playwright/test";

/**
 * Global setup: log in once as admin and save the browser storage state
 * so all test workers can reuse the session without hitting rate limits.
 */
async function globalSetup(config: FullConfig) {
  const baseURL =
    config.projects[0].use.baseURL || "http://webapp:8080";

  const browser = await chromium.launch();
  const context = await browser.newContext({ baseURL });
  const page = await context.newPage();

  await page.goto("/login");
  await page.fill('input[name="username"]', "testadmin");
  await page.fill('input[name="password"]', "testpassword123");
  await page.click('button[type="submit"]');
  await page.waitForURL("**/admin/league/dashboard");

  // Save the authenticated state
  await context.storageState({ path: "auth-state.json" });
  await browser.close();
}

export default globalSetup;
