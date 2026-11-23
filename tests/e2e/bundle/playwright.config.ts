import { defineConfig } from "@playwright/test";

// The bundle smoke test (WI-147): its own config, so the existing suites are
// unaffected (the main config ignores this directory). One serial flow per
// project against one freshly started instance of the SHIPPED deployment:
//   bundle-on   tournaments profile on   (01-*.spec.ts ... in file-name order)
//   bundle-off  tournaments profile off  (off.spec.ts)
// Run it through scripts/test-e2e-bundle.sh (`make test-e2e-bundle`).
export default defineConfig({
  testDir: ".",
  testMatch: "**/*.spec.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  forbidOnly: !!process.env.CI,
  timeout: 120_000,
  expect: { timeout: 15_000 },
  outputDir: "test-results",
  reporter: [["list"], ["html", { open: "never", outputFolder: "playwright-report" }]],
  use: {
    baseURL: process.env.BUNDLE_URL || "https://localhost:18743",
    // Caddy's internal CA is not trusted by the browser.
    ignoreHTTPSErrors: true,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    actionTimeout: 20_000,
    navigationTimeout: 30_000,
  },
  projects: [
    { name: "bundle-on", testIgnore: "**/off.spec.ts", use: { browserName: "chromium" } },
    { name: "bundle-off", testMatch: "**/off.spec.ts", use: { browserName: "chromium" } },
  ],
});
